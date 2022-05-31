package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/mitranim/gg"
)

func OptDefault() Opt { return gg.FlagParseTo[Opt](nil) }

type Opt struct {
	Args         []string         `flag:""`
	Help         bool             `flag:"-h"               desc:"Print help and exit."`
	Cmd          string           `flag:"-g" init:"go"     desc:"Verbose logging."`
	Verb         bool             `flag:"-v"               desc:"Go tool to use."`
	ClearHard    bool             `flag:"-c"               desc:"Clear terminal on restart."`
	ClearSoft    bool             `flag:"-s"               desc:"Soft-clear terminal, keeping scrollback."`
	Raw          bool             `flag:"-r" init:"true"   desc:"Enable terminal raw mode and hotkeys."`
	Sep          FlagStrMultiline `flag:"-S"               desc:"Separator printed after each run; multi; supports \\n."`
	Trace        bool             `flag:"-t"               desc:"Print error trace on exit. Useful for debugging gow."`
	Extensions   FlagExtensions   `flag:"-e" init:"go,mod" desc:"Extensions to watch; multi."`
	Watch        FlagWatch        `flag:"-w" init:"."      desc:"Paths to watch, relative to CWD; multi."`
	IgnoredPaths FlagIgnoredPaths `flag:"-i"               desc:"Ignored paths, relative to CWD; multi."`
}

func (self *Opt) Init(src []string) {
	err := gg.FlagParseCatch(src, self)
	if err != nil {
		self.LogErr(err)
		gg.Write(log.Writer(), gg.Newline)
		self.PrintHelp()
		os.Exit(1)
	}

	if self.Help || gg.Head(self.Args) == `help` {
		self.PrintHelp()
		os.Exit(0)
	}

	if gg.IsEmpty(self.Args) {
		self.PrintHelp()
		os.Exit(1)
	}
}

func (self Opt) PrintHelp() {
	gg.FlagFmtDefault.Prefix = "\t"
	gg.FlagFmtDefault.Head = false

	gg.Nop2(fmt.Fprintf(os.Stderr, `"gow" is the missing watch mode for the "go" command.
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

%v
"Multi" flags can be passed multiple times.
Some also support comma-separated parsing.

Supported control codes / hotkeys:

	3     ^C          Kill subprocess or self with SIGINT.
	18    ^R          Kill subprocess with SIGTERM, restart.
	20    ^T          Kill subprocess with SIGTERM.
	28    ^\          Kill subprocess or self with SIGQUIT.
	31    ^- or ^?    Print currently running command.
`, gg.FlagHelp[Opt]()))
}

func (self Opt) LogErr(err error) {
	if err != nil {
		if self.Trace {
			log.Printf(`%+v`, err)
		} else {
			log.Println(err)
		}
	}
}

func (self Opt) TermClear() {
	if self.ClearHard {
		gg.TermClearHard()
	} else if self.ClearSoft {
		gg.TermClearSoft()
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
