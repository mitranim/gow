package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/mitranim/gg"
)

type Opt struct {
	flag.FlagSet
	Args         []string
	Cmd          string
	Verb         bool
	ClearHard    bool
	ClearSoft    bool
	Raw          bool
	Sep          FlagStrMultiline
	Extensions   FlagExtensions
	IgnoredPaths FlagIgnoredPaths
	Watch        FlagWatch
}

func (self *Opt) Init() {
	self.FlagSet.Init(os.Args[0], flag.ExitOnError)

	self.StringVar(&self.Cmd, `g`, `go`, ``)
	self.BoolVar(&self.Verb, `v`, false, ``)
	self.BoolVar(&self.ClearHard, `c`, false, ``)
	self.BoolVar(&self.ClearSoft, `s`, false, ``)
	self.BoolVar(&self.Raw, `r`, true, ``)
	self.Var(&self.Sep, `S`, ``)
	self.Var(&self.Extensions, `e`, ``)
	self.Var(&self.IgnoredPaths, `i`, ``)
	self.Var(&self.Watch, `w`, ``)

	self.Extensions.Default()
	self.Watch.Default()
	self.Usage = self.PrintHelp
}

func (self *Opt) Parse() {
	gg.Try(self.FlagSet.Parse(os.Args[1:]))

	self.Args = self.FlagSet.Args()
	if gg.IsEmpty(self.Args) {
		self.Usage()
		os.Exit(1)
	}
}

func (self Opt) PrintHelp() {
	gg.Nop2(fmt.Fprintf(self.Output(), `"gow" is the missing watch mode for the "go" command.
Runs an arbitrary "go" subcommand, watches files, and restarts on changes.

Usage:

	gow <gow_flags> <cmd> <cmd_flags> <cmd_args ...>

Examples:

	gow    -v -c          test     -v -count=1    .
	       ↑ gow_flags    ↑ cmd    ↑ cmd_flags    ↑ cmd_args

	gow run . a b c
	gow -v -c -e=go,mod,html run .
	gow -v -c test
	gow -v -c vet
	gow -v -c install

Flags:

	-h    Print help and exit.
	-v    Verbose logging.
	-c    Clear terminal on restart.
	-s    Soft-clear terminal, keeping scrollback.
	-e    Extensions to watch, comma-separated; default: %[1]q.
	-i    Ignored paths, relative to CWD, comma-separated.
	-w    Paths to watch, relative to CWD, comma-separated; default: %[2]q.
	-r    Enable terminal raw mode and hotkeys; default: %[3]v.
	-g    The Go tool to use; default: %[4]q.
	-S    Separator string printed after each run; supports "\n"; default: "%[5]v".

Supported control codes / hotkeys:

	3     ^C          Kill subprocess or self with SIGINT.
	18    ^R          Kill subprocess with SIGTERM, restart.
	20    ^T          Kill subprocess with SIGTERM.
	28    ^\          Kill subprocess or self with SIGQUIT.
	31    ^- or ^?    Print currently running command.
`, self.Extensions.String(), self.Watch.String(), self.Raw, self.Cmd, self.Sep))
}

func (self Opt) TermClear() {
	if self.ClearHard {
		gg.TermClearHard()
	} else if self.ClearSoft {
		gg.TermClearHard()
	}
}

func (self Opt) MakeCmd() *exec.Cmd {
	cmd := exec.Command(self.Cmd, self.Args...)

	// Causes the OS to assign process group ID = `cmd.Process.Pid`.
	// We use this to broadcast signals to the entire subprocess group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (self Opt) ShouldRestart(event FsEvent) bool {
	if event == nil {
		return false
	}
	path := event.Path()
	return self.IgnoredPaths.Allow(path) && self.Extensions.Allow(path)
}
