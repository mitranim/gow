package main

import (
	"flag"
	l "log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rjeczalik/notify"
)

// Consider making configurable?
const CMD = "go"

const HELP = `"gow" is a watch mode for the "go" command. Run an arbitrary subcommand,
watch Go files in the same directory, and restart on changes.

Note that running a directory with "go run ." requires Go 1.11 or higher.

Usage:

	gow <subcommand> <args ...>

The first argument can be ANY subcommand for "go":

	gow run . arg0 arg1 arg2
	gow vet
	gow build -o ./my-program
	gow install
	...

Options:

	-v	Verbose logging
`

var (
	log     = l.New(os.Stderr, "[gow] ", 0)
	VERBOSE = flag.Bool("v", false, "")
)

func main() {
	flag.Usage = printHelp

	// In addition to parsing, this will also print help and exit when called
	// with "-h".
	flag.Parse()

	args := flag.Args()

	if len(args) < 1 {
		printHelp()
		os.Exit(1)
	}

	err := watchAndRerun(args)
	if err != nil {
		log.Fatal(err)
	}
}

func printHelp() {
	os.Stderr.Write([]byte(HELP))
}

func watchAndRerun(args []string) error {
	events := make(chan notify.EventInfo, 1)

	// On MacOS, this should be fairly efficient regardless of directory/file
	// count. Might be expensive on other systems. Needs feedback.
	const pattern = `./...`

	err := notify.Watch(pattern, events, notify.All)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd

	// Broadcast common "stop" signals to the child/grandchild process group.
	// Known problem: can't catch SIGKILL.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		sig := (<-sigs).(syscall.Signal)
		signal.Stop(sigs)
		if cmd != nil {
			_ = syscall.Kill(-cmd.Process.Pid, sig)
		}
		_ = syscall.Kill(os.Getpid(), sig)
	}()

	for {
		cmd = exec.Command(CMD, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Causes the OS to assign process group ID = `cmd.Process.Pid`.
		// We use this to kill the grandchild process, if any.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		done := make(chan struct{})
		go func() {
			defer close(done)

			err := cmd.Start()
			if err != nil {
				log.Printf("Failed to start: %+v", err)
				return
			}

			// If killed, this looks something like "signal: killed".
			err = cmd.Wait()

			if *VERBOSE {
				if err != nil {
					if args[0] == "run" && strings.Contains(err.Error(), "exit status") {
						// Suppress "exit status N" for `go run`: it already
						// prints the program's exit code to stderr, and logging
						// its own exit code is pointless.
					} else {
						log.Println("Subcommand error:", err)
					}
				} else {
					log.Println("exit ok")
				}
			}
		}()

		for event := range events {
			if shouldIgnore(event) {
				continue
			}
			if *VERBOSE {
				log.Println("Event:", event)
			}

			// Might be unsafe on architectures with non-atomic word-sized reads
			proc := cmd.Process
			if proc != nil {
				// cmd's process group equals its `cmd.Process.Pid` (see above).
				// Passing a negative value causes it to be treated as a group
				// ID. This should broadcast the signal to the child AND
				// grandchild processes, if any.
				_ = syscall.Kill(-proc.Pid, syscall.SIGKILL)
			}

			// Might be unsafe on architectures with non-atomic word-sized reads
			if *VERBOSE && cmd.ProcessState == nil {
				log.Println("Waiting for subcommand")
			}
			<-done

			break
		}
	}
}

func shouldIgnore(event notify.EventInfo) bool {
	path := event.Path()
	return filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go")
}
