package main

import (
	"flag"
	"io"
	l "log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rjeczalik/notify"
	"golang.org/x/sys/unix"
)

// Consider making configurable?
const CMD = "go"

const HELP = `"gow" is the missing watch mode for the "go" command.

Runs an arbitrary "go" subcommand, watches Go files, and restarts on changes.
Note that running a directory with "go run ." requires Go 1.11 or higher.

Usage:

	gow <flags> <subcommand> <flags> <args ...>

	gow run . a b c
	gow test
	gow -v vet
	gow -v install

	gow -v           build          -o ./my-program
	    ↑ gow flag   ↑ subcommand   ↑ subcommand flag

Options:

	-v	Verbose logging
	-c	Clear terminal on restart
	-s	Soft-clear terminal on restart; keeps scrollback

Supported control codes:

	3	^C	kill subprocess or self with SIGINT
	18	^R	kill subprocess with SIGTERM, restart
	20	^T	kill subprocess with SIGTERM
	28	^\	kill subprocess or self with SIGQUIT

`

const (
	ASCII_END_OF_TEXT      = 3  // ^C
	ASCII_FILE_SEPARATOR   = 28 // ^\
	ASCII_DEVICE_CONTROL_2 = 18 // ^R
	ASCII_DEVICE_CONTROL_4 = 20 // ^T

	CODE_INTERRUPT = ASCII_END_OF_TEXT
	CODE_QUIT      = ASCII_FILE_SEPARATOR
	CODE_RESTART   = ASCII_DEVICE_CONTROL_2
	CODE_STOP      = ASCII_DEVICE_CONTROL_4

	TERM_CLEAR_SOFT       = "\x1bc"
	TERM_CLEAR_SCROLLBACK = "\x1b[3J"
	TERM_CLEAR_HARD       = TERM_CLEAR_SOFT + TERM_CLEAR_SCROLLBACK
)

var (
	log        = l.New(os.Stderr, "[gow] ", 0)
	VERBOSE    = flag.Bool("v", false, "")
	CLEAR_HARD = flag.Bool("c", false, "")
	CLEAR_SOFT = flag.Bool("s", false, "")
)

func main() {
	flag.Usage = func() { os.Stderr.Write([]byte(HELP)) }

	// Side effects:
	//   * parse flags into previously-declared vars
	//   * when called with undefined flags, print error + help and exit
	//   * when called with "-h", print help and exit
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Everything needed for cleanup must be registered here.
	var termios *unix.Termios
	var signals chan os.Signal
	var cmd *exec.Cmd

	// This MUST be called before exit. Can't rely on "defer" because some OS
	// signals may kill our program without running deferred calls.
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
	state, err := makeTerminalRaw(syscall.Stdin)
	critical(err)
	termios = &state

	/**
	Signal handling

	We override Go's default signal handling to ensure cleanup before exit.
	Cleanup is necessary to restore the previous terminal state and kill any
	sub-sub-processes.
	*/
	signals = make(chan os.Signal, 1)
	signal.Notify(signals)

	/**
	FS watching

	On MacOS, recursive watching should be fairly efficient regardless of
	directory and file count. Might be expensive on other systems. Needs
	feedback.
	*/
	fsEvents := make(chan notify.EventInfo, 1)
	err = notify.Watch(`./...`, fsEvents, notify.All)
	critical(err)

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
		cmd, cmdStdin, err = makeSubcommand(args)
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

		// Could be declared at the point of use, but escape analysis may move
		// this to the heap, making each write more expensive than it should be.
		var stdinBuf [1]byte

	progress:
		for {
			select {
			case err := <-cmdErr:
				if err != nil {
					// `go run` reports the program's exit code to stderr;
					// suppress the error message in this case.
					alreadyReported := args[0] == "run" && strings.Contains(err.Error(), "exit status")
					if !alreadyReported {
						log.Printf("subcommand error: %v", err)
					}
				} else if *VERBOSE {
					log.Println("exit ok")
				}
				cmd = nil
				cmdStdin = nil

			case signal := <-signals:
				sig := signal.(syscall.Signal)

				switch sig {
				case syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM:
					_ = broadcastSignal(cmd, sig)
					cmd = nil
					cleanup()
					// This MAY kill our process.
					_ = syscall.Kill(os.Getpid(), sig)
					// In case it didn't, suicide to avoid being stuck in limbo.
					os.Exit(1)

				// Pass; we report child exit status separately.
				case syscall.SIGCHLD:

				// Pass; uninteresting spam.
				case syscall.SIGWINCH:

				default:
					if *VERBOSE {
						log.Println("received signal:", sig)
					}
				}

			case fsEvent := <-fsEvents:
				if shouldIgnore(fsEvent) {
					continue progress
				}
				if *VERBOSE {
					log.Println("restarting on FS event:", fsEvent)
				}
				goto restart

			// Probably much slower than reading and writing synchronously on a
			// single goroutine. TODO figure out how to avoid channels.
			case char := <-stdin:
				// Interpret known ASCII codes into OS signals, otherwise
				// forward the input to the subprocess.
				switch char {
				case CODE_INTERRUPT:
					if cmd == nil {
						if *VERBOSE {
							log.Println("received ^C, shutting down")
						}
						cleanup()
						syscall.Kill(os.Getpid(), syscall.SIGINT)
					} else if lastChar == CODE_INTERRUPT && time.Now().Sub(lastInst) < time.Second {
						if *VERBOSE {
							log.Println("received ^C^C, shutting down")
						}
						cleanup()
						syscall.Kill(os.Getpid(), syscall.SIGINT)
					} else {
						if *VERBOSE {
							log.Println("received ^C, stopping subprocess")
						}
						_ = broadcastSignal(cmd, syscall.SIGINT)
					}

				case CODE_QUIT:
					if cmd == nil {
						if *VERBOSE {
							log.Println("received ^\\, shutting down")
						}
						cleanup()
						syscall.Kill(os.Getpid(), syscall.SIGQUIT)
					} else {
						if *VERBOSE {
							log.Println("received ^\\, stopping subprocess")
						}
						_ = broadcastSignal(cmd, syscall.SIGQUIT)
					}

				case CODE_RESTART:
					if *VERBOSE {
						log.Println("received ^R, restarting")
					}
					goto restart

				case CODE_STOP:
					if cmd == nil {
						if *VERBOSE {
							log.Println("received ^T, nothing to stop")
						}
					} else {
						if *VERBOSE {
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
		_ = broadcastSignal(cmd, syscall.SIGTERM)
		if *CLEAR_HARD {
			os.Stdout.Write([]byte(TERM_CLEAR_HARD))
		} else if *CLEAR_SOFT {
			os.Stdout.Write([]byte(TERM_CLEAR_SOFT))
		}
	}
}

/**
References:
	https://en.wikibooks.org/wiki/Serial_Programming/termios
	man termios
*/
func makeTerminalRaw(fd int) (unix.Termios, error) {
	prev, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
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

	// Seems unnecessary on my system. Might be needed for others.
	// termios.Cflag |= unix.CS8
	// termios.Cc[unix.VMIN] = 1
	// termios.Cc[unix.VTIME] = 0

	err = unix.IoctlSetTermios(fd, unix.TIOCSETA, &termios)
	return *prev, err
}

func restoreTerminal(fd int, termios unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, &termios)
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
	cmd := exec.Command(CMD, args...)

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

func shouldIgnore(event notify.EventInfo) bool {
	return filepath.Ext(event.Path()) != ".go"
}
