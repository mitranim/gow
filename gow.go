/*
Go Watch: missing watch mode for the "go" command. It's invoked exactly like
"go", but also watches Go files and reruns on changes.

See the readme at https://github.com/mitranim/gow.
*/
package main

import (
	"flag"
	"fmt"
	"io"
	l "log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/rjeczalik/notify"
	"golang.org/x/sys/unix"
)

var HELP = fmt.Sprintf(`"gow" is the missing watch mode for the "go" command.
Runs an arbitrary "go" subcommand, watches files, and restarts on changes.

Usage:

	gow <flags> <subcommand> <flags> <args ...>

Examples:

	gow   -v -c         test           -v -count=1          .
	      ^ gow flags   ^ subcommand   ^ subcommand flags   ^ subcommand args

	gow run . a b c
	gow -v -c -e=go,mod,html run .
	gow -v -c test
	gow -v -c vet
	gow -v -c install

Options:

	-v    Verbose logging
	-c    Clear terminal on restart
	-s    Soft-clear terminal, keeping scrollback
	-e    Extensions to watch, comma-separated; default: %[1]q
	-i    Ignored paths, relative to CWD, comma-separated
	-w    Paths to watch, relative to CWD, comma-separated; default: %[2]q
	-r    Enable terminal raw mode and hotkeys; default: %[3]v
	-g    The Go tool to use; default: %[4]q
	-S    Separator string printed after each run; supports \n; default: "%[5]v"

Supported control codes / hotkeys:

	3     ^C          kill subprocess or self with SIGINT
	18    ^R          kill subprocess with SIGTERM, restart
	20    ^T          kill subprocess with SIGTERM
	28    ^\          kill subprocess or self with SIGQUIT
	31    ^- or ^?    print currently running command
`, EXTENSIONS, WATCH, *FLAG_RAW, *FLAG_CMD, *FLAG_SEP)

const (
	ASCII_END_OF_TEXT      = 3  // ^C
	ASCII_FILE_SEPARATOR   = 28 // ^\
	ASCII_DEVICE_CONTROL_2 = 18 // ^R
	ASCII_DEVICE_CONTROL_4 = 20 // ^T
	ASCII_UNIT_SEPARATOR   = 31 // ^- or ^?

	CODE_INTERRUPT     = ASCII_END_OF_TEXT
	CODE_QUIT          = ASCII_FILE_SEPARATOR
	CODE_RESTART       = ASCII_DEVICE_CONTROL_2
	CODE_STOP          = ASCII_DEVICE_CONTROL_4
	CODE_PRINT_COMMAND = ASCII_UNIT_SEPARATOR

	ESC                   = "\x1b"
	TERM_CLEAR_SOFT       = ESC + "c"
	TERM_CLEAR_SCROLLBACK = ESC + "[3J"
	TERM_CLEAR_HARD       = TERM_CLEAR_SOFT + TERM_CLEAR_SCROLLBACK

	NEWLINE = "\n"
)

var (
	FLAG_SET        = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	FLAG_CMD        = FLAG_SET.String("g", "go", "")
	FLAG_VERBOSE    = FLAG_SET.Bool("v", false, "")
	FLAG_CLEAR_HARD = FLAG_SET.Bool("c", false, "")
	FLAG_CLEAR_SOFT = FLAG_SET.Bool("s", false, "")
	FLAG_RAW        = FLAG_SET.Bool("r", true, "")
	FLAG_SEP        = FLAG_SET.String("S", "", "")
	SEP             []byte

	EXTENSIONS    = &flagStrings{validateExtension, decorateExtension, []string{"go", "mod"}}
	IGNORED_PATHS = &flagStrings{validatePath, decorateIgnore, nil}
	WATCH         = &flagStrings{validatePath, nil, DEFAULT_WATCH}
	DEFAULT_WATCH = []string{`.`}

	log         = l.New(os.Stderr, "[gow] ", 0)
	killSignals = []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM}
)

func main() {
	FLAG_SET.Usage = func() {}
	FLAG_SET.Var(EXTENSIONS, "e", "")
	FLAG_SET.Var(IGNORED_PATHS, "i", "")
	FLAG_SET.Var(WATCH, "w", "")

	err := FLAG_SET.Parse(os.Args[1:])

	if err == flag.ErrHelp {
		printHelp(FLAG_SET.Output())
		os.Exit(1)
	} else if err != nil {
		// Note: `flag` automatically prints this error to `FLAG_SET.Output()`.
		os.Exit(2)
	}

	subArgs := FLAG_SET.Args()
	if len(subArgs) == 0 {
		printHelp(FLAG_SET.Output())
		os.Exit(1)
	}

	SEP = []byte(unescapedLine(*FLAG_SEP))

	EXTENSIONS.Prepare()
	IGNORED_PATHS.Prepare()

	// Everything needed for cleanup must be registered here.
	var termios *unix.Termios
	var cmd *exec.Cmd
	signals := make(chan os.Signal, 1)

	/**
	This MUST be called manually before exiting. The current implementation of
	`gow` can't rely on `defer` for cleanup because it self-terminates by calling
	`os.Exit` or broadcasting OS kill signals, which may kill the program without
	running deferred calls.
	*/
	cleanup := func() {
		if termios != nil {
			_ = restoreTerminal(syscall.Stdin, *termios)
		}
		if cmd != nil {
			_ = broadcastSignal(cmd, syscall.SIGTERM)
			cmd = nil
		}
		if signals != nil {
			signal.Stop(signals)
		}
	}

	critical := func(err error) {
		if err != nil {
			cleanup()
			log.Println(err)
			os.Exit(1)
		}
	}

	/**
	Terminal state

	By default, the terminal uses what's known as "cooked mode". It buffers
	lines before sending them to the program, and interprets some ASCII control
	codes on stdin by sending the corresponding OS signals to our process. We
	switch it into "raw mode", where it immediately forwards inputs to our
	process's stdin, and doesn't interpret special ASCII codes. This is done to
	support special key combinations such as ^R for restarting a subprocess.

	The terminal state is shared between all super- and sub-processes. Changes
	persist even after "gow" terminates. We endeavor to restore the previous
	state before exiting.
	*/
	if *FLAG_RAW {
		state, err := makeTerminalRaw(syscall.Stdin)
		if err != nil {
			log.Printf("failed to set raw terminal mode: %+v", err)
		} else {
			termios = &state
		}
	}

	/**
	Signal handling

	We override Go's default signal handling to ensure cleanup before exit.
	Cleanup is necessary to restore the previous terminal state and kill any
	sub-sub-processes.

	The set of signals registered here MUST match the set of signals explicitly
	handled by this program; see below.
	*/
	signal.Notify(signals, killSignals...)

	/**
	FS watching

	On MacOS, recursive watching should be fairly efficient regardless of
	directory and file count. Might be expensive on other systems. Needs
	feedback.
	*/
	fsEvents := make(chan notify.EventInfo, 1)
	watch := WATCH.values
	reportWatch := !reflect.DeepEqual(DEFAULT_WATCH, watch)
	for _, val := range watch {
		val = filepath.Join(val, `...`)
		if reportWatch && *FLAG_VERBOSE {
			log.Printf(`watching %q`, val)
		}
		critical(notify.Watch(val, fsEvents, notify.All))
	}

	/**
	Stdin handling

	See the terminal setup above. The "raw mode" allows us to support our own
	control codes, but we're also responsible for interpreting common ASCII
	codes into OS signals.
	*/
	stdin := make(chan byte, 1024*4)
	go readStdin(stdin)

	for {
		// Setup and start subprocess
		cmdErr := make(chan error, 1)

		var cmdStdin io.WriteCloser
		cmd, cmdStdin, err = makeSubcommand(subArgs)
		if err != nil {
			log.Printf("failed to initialize subcommand: %v", err)
		} else {
			// Wait until the subprocess starts, populating `cmd.Process`.
			// We need it for killing the subprocess group.
			err := cmd.Start()
			if err != nil {
				log.Printf("failed to start subcommand: %v", err)
				cmd = nil
				cmdStdin = nil
			} else {
				cmd := cmd
				go func() { cmdErr <- cmd.Wait() }()
			}
		}

		// Used to detect double ^C; see below.
		var lastChar byte
		var lastInst time.Time

		// Could be declared at the point of use, but escape analysis may
		// spuriously move it to the heap. Don't want to make writes more
		// expensive than they should be.
		var stdinBuf [1]byte

	progress:
		for {
			select {
			case err := <-cmdErr:
				if err != nil {
					// `go run` reports the program's exit code to stderr;
					// suppress the error message in this case.
					alreadyReported := subArgs[0] == "run" && strings.Contains(err.Error(), "exit status")
					if !alreadyReported {
						log.Printf("subcommand error: %v", err)
					}
				} else if *FLAG_VERBOSE {
					log.Println("exit ok")
				}

				if len(SEP) > 0 {
					log.Writer().Write(SEP)
				}
				cmd = nil
				cmdStdin = nil

			case signal := <-signals:
				sig := signal.(syscall.Signal)

				if signalsInclude(killSignals, sig) {
					_ = broadcastSignal(cmd, sig)
					cmd = nil
					cleanup()
					// This should kill our process.
					_ = syscall.Kill(os.Getpid(), sig)
					// Suicide just in case.
					os.Exit(1)
				} else {
					if *FLAG_VERBOSE {
						log.Println("received signal:", sig)
					}
				}

			case fsEvent := <-fsEvents:
				allowed, err := shouldRestart(fsEvent)

				if err != nil {
					if *FLAG_VERBOSE {
						log.Printf("ignoring FS event %v: %v", fsEvent, err)
					}
					continue progress
				}

				if !allowed {
					continue progress
				}

				if *FLAG_VERBOSE {
					log.Println("restarting on FS event:", fsEvent)
				}

				goto restart

			// Probably slower than reading and writing synchronously on
			// a single goroutine. Unclear how to avoid channels.
			case char := <-stdin:
				// Interpret known ASCII codes into OS signals, otherwise
				// forward the input to the subprocess.
				switch char {
				case CODE_INTERRUPT:
					if cmd == nil {
						if *FLAG_VERBOSE {
							log.Println("received ^C, shutting down")
						}
						cleanup()
						syscall.Kill(os.Getpid(), syscall.SIGINT)
					} else if lastChar == CODE_INTERRUPT && time.Now().Sub(lastInst) < time.Second {
						if *FLAG_VERBOSE {
							log.Println("received ^C^C, shutting down")
						}
						cleanup()
						syscall.Kill(os.Getpid(), syscall.SIGINT)
					} else {
						if *FLAG_VERBOSE {
							log.Println("received ^C, stopping subprocess")
						}
						_ = broadcastSignal(cmd, syscall.SIGINT)
					}

				case CODE_QUIT:
					if cmd == nil {
						if *FLAG_VERBOSE {
							log.Println("received ^\\, shutting down")
						}
						cleanup()
						syscall.Kill(os.Getpid(), syscall.SIGQUIT)
					} else {
						if *FLAG_VERBOSE {
							log.Println("received ^\\, stopping subprocess")
						}
						_ = broadcastSignal(cmd, syscall.SIGQUIT)
					}

				case CODE_PRINT_COMMAND:
					log.Printf("current command: %q\n", os.Args)

				case CODE_RESTART:
					if *FLAG_VERBOSE {
						log.Println("received ^R, restarting")
					}
					goto restart

				case CODE_STOP:
					if cmd == nil {
						if *FLAG_VERBOSE {
							log.Println("received ^T, nothing to stop")
						}
					} else {
						if *FLAG_VERBOSE {
							log.Println("received ^T, stopping")
						}
						_ = broadcastSignal(cmd, syscall.SIGTERM)
					}

				default:
					stdinBuf[0] = char
					if cmdStdin != nil {
						cmdStdin.Write(stdinBuf[:])
					}
				}

				lastChar = char
				lastInst = time.Now()
			}
		}

	restart:
		clearTerminal()
		_ = broadcastSignal(cmd, syscall.SIGTERM)
	}
}

func printHelp(writer io.Writer) {
	writer.Write([]byte(HELP))
}

/**
References:
	https://en.wikibooks.org/wiki/Serial_Programming/termios
	man termios
*/
func makeTerminalRaw(fd int) (unix.Termios, error) {
	prev, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return unix.Termios{}, err
	}

	termios := *prev

	// Don't buffer lines
	termios.Lflag &^= unix.ICANON

	// Don't echo characters or special codes.
	termios.Lflag &^= unix.ECHO

	// No signals
	termios.Lflag &^= unix.ISIG

	// Seems unnecessary on my system. Might be needed elsewhere.
	// termios.Cflag |= unix.CS8
	// termios.Cc[unix.VMIN] = 1
	// termios.Cc[unix.VTIME] = 0

	err = unix.IoctlSetTermios(fd, ioctlWriteTermios, &termios)
	return *prev, err
}

func restoreTerminal(fd int, termios unix.Termios) error {
	return unix.IoctlSetTermios(fd, ioctlWriteTermios, &termios)
}

func readStdin(out chan<- byte) {
	var buf [1]byte
	for {
		n, err := os.Stdin.Read(buf[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		if n == 0 {
			continue
		}
		out <- buf[0]
	}
}

func makeSubcommand(args []string) (*exec.Cmd, io.WriteCloser, error) {
	cmd := exec.Command(*FLAG_CMD, args...)

	// Causes the OS to assign process group ID = `cmd.Process.Pid`.
	// We use this to broadcast signals to the entire subprocess group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmdStdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	return cmd, cmdStdin, nil
}

// Sends the signal to the subprocess group, denoted by the negative sign on the
// PID. Note that this works only with `Setpgid`.
func broadcastSignal(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd != nil {
		proc := cmd.Process
		if proc != nil {
			return syscall.Kill(-proc.Pid, sig)
		}
	}
	return nil
}

func shouldRestart(fsEvent notify.EventInfo) (bool, error) {
	absPath := fsEvent.Path()

	ok, err := allowByIgnoredPaths(absPath)
	if err != nil || !ok {
		return ok, err
	}

	return allowByExtensions(absPath), nil
}

func allowByIgnoredPaths(absPath string) (bool, error) {
	if len(IGNORED_PATHS.values) == 0 {
		return true, nil
	}

	for _, ignored := range IGNORED_PATHS.values {
		if hasBasePath(absPath, ignored) {
			return false, nil
		}
	}

	return true, nil
}

// Assumes both paths are relative or both are absolute. Doesn't care to support
// scheme-qualified paths such as network paths.
func hasBasePath(longerPath string, basePath string) bool {
	return strings.HasPrefix(longerPath, basePath)
}

func allowByExtensions(path string) bool {
	ext := filepath.Ext(path)
	// Note: `filepath.Ext` includes a dot prefix, which we have to slice off.
	return ext != "" && stringsInclude(EXTENSIONS.values, ext)
}

func stringsInclude(list []string, val string) bool {
	for _, elem := range list {
		if elem == val {
			return true
		}
	}
	return false
}

type flagStrings struct {
	validate func(string) error
	decorate func(string) string
	values   []string
}

func (self *flagStrings) String() string {
	if self != nil {
		return strings.Join(self.values, ",")
	}
	return ""
}

func (self *flagStrings) Set(input string) error {
	vals := strings.Split(input, ",")

	for _, val := range vals {
		err := self.validate(val)
		if err != nil {
			return err
		}
	}

	self.values = vals
	return nil
}

func (self *flagStrings) Prepare() {

	var values []string
	for _, val := range self.values {
		if self.decorate != nil {
			val = self.decorate(val)
		}
		values = append(values, val)
	}

	self.values = values
}

func validateExtension(val string) error {
	if wordRegexp.MatchString(val) {
		return nil
	}
	return fmt.Errorf(`invalid extension %q`, val)
}

var wordRegexp = regexp.MustCompile(`^\w+$`)

// TODO remove. Seems completely unnecessary.
func validatePath(val string) error {
	if pathRegexp.MatchString(val) {
		return nil
	}
	return fmt.Errorf(
		`FS path %q appears to be invalid; must match regexp %q`,
		val, pathRegexp,
	)
}

func decorateExtension(val string) string {
	return fmt.Sprintf(".%s", val)
}

func decorateIgnore(val string) string {
	if !filepath.IsAbs(val) {
		cwd, _ := os.Getwd()
		val = filepath.Join(cwd, val)
	}

	return filepath.Clean(val)
}

var pathRegexp = regexp.MustCompile(`^[\w. /\\-]+$`)

func clearTerminal() {
	if *FLAG_CLEAR_HARD {
		os.Stdout.Write([]byte(TERM_CLEAR_HARD))
	} else if *FLAG_CLEAR_SOFT {
		os.Stdout.Write([]byte(TERM_CLEAR_SOFT))
	}
}

func signalsInclude(signals []os.Signal, sig os.Signal) bool {
	for _, signal := range signals {
		if signal == sig {
			return true
		}
	}
	return false
}

func unescapedLine(str string) string {
	str = strings.ReplaceAll(str, `\n`, NEWLINE)
	if len(str) > 0 && !strings.HasSuffix(str, NEWLINE) {
		str += NEWLINE
	}
	return str
}
