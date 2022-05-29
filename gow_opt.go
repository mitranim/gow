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
	Watch        FlagWatch
	IgnoredPaths FlagIgnoredPaths
}

func (self *Opt) Init() {
	self.FlagSet.Init(os.Args[0], flag.ExitOnError)

	self.StringVar(&self.Cmd, `g`, DEFAULT_CMD, ``)
	self.BoolVar(&self.Verb, `v`, false, ``)
	self.BoolVar(&self.ClearHard, `c`, false, ``)
	self.BoolVar(&self.ClearSoft, `s`, false, ``)
	self.BoolVar(&self.Raw, `r`, DEFAULT_RAW, ``)
	self.Var(&self.Sep, `S`, ``)
	self.Var(&self.Extensions, `e`, ``)
	self.Var(&self.Watch, `w`, ``)
	self.Var(&self.IgnoredPaths, `i`, ``)
}

func (self *Opt) Parse() {
	self.Usage = self.PrintHelp
	gg.Try(self.FlagSet.Parse(os.Args[1:]))

	self.Extensions.Default()
	self.Watch.Default()

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

	gow    -c -v          test     -v -count=1    .
	       ↑ gow_flags    ↑ cmd    ↑ cmd_flags    ↑ cmd_args

	gow                             run . a b c
	gow -c -v -e=go -e=mod -e=html  run .
	gow -c -v                       test
	gow -c -v                       install
	gow -c -v -w=src -i=.git -i=tar vet

Flags:

	-h    Print help and exit.
	-v    Verbose logging.
	-g    Go tool to use; default: %[1]q.
	-c    Clear terminal on restart.
	-s    Soft-clear terminal, keeping scrollback.
	-r    Enable terminal raw mode and hotkeys; default: %[2]v.
	-S    Separator string printed after each run; multi; supports "\n".
	-e    Extensions to watch; multi; default: %[3]q.
	-w    Paths to watch, relative to CWD; multi; default: %[4]q.
	-i    Ignored paths, relative to CWD; multi.

"Multi" flags can be passed multiple times.
In addition, some flags support comma-separated parsing.

Supported control codes / hotkeys:

	3     ^C          Kill subprocess or self with SIGINT.
	18    ^R          Kill subprocess with SIGTERM, restart.
	20    ^T          Kill subprocess with SIGTERM.
	28    ^\          Kill subprocess or self with SIGQUIT.
	31    ^- or ^?    Print currently running command.
`,
		DEFAULT_CMD,
		DEFAULT_RAW,
		DEFAULT_WATCH,
		DEFAULT_EXTENSIONS,
	))
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
