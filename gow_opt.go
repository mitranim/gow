package main

import (
	e "errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/mitranim/gg"
)

func OptDefault() Opt { return gg.FlagParseTo[Opt](nil) }

type Opt struct {
	Args         []string         `flag:""`
	Help         bool             `flag:"-h"                desc:"Print help and exit."`
	Cmd          string           `flag:"-g"  init:"go"     desc:"Go tool to use."`
	Verb         bool             `flag:"-v"                desc:"Verbose logging."`
	ClearHard    bool             `flag:"-c"                desc:"Clear terminal on restart."`
	ClearSoft    bool             `flag:"-s"                desc:"Soft-clear terminal, keeping scrollback."`
	Raw          bool             `flag:"-r"  init:"true"   desc:"Enable terminal raw mode and hotkeys."`
	Pre          FlagStrMultiline `flag:"-P"                desc:"Prefix printed BEFORE each run; multi; supports \\n."`
	Suf          FlagStrMultiline `flag:"-S"                desc:"Suffix printed AFTER each run; multi; supports \\n."`
	Trace        bool             `flag:"-t"                desc:"Print error trace on exit. Useful for debugging gow."`
	RawEcho      bool             `flag:"-re" init:"true"   desc:"In raw mode, echo stdin to stdout like most apps."`
	Lazy         bool             `flag:"-l"                desc:"Lazy mode: restart only when subprocess is not running."`
	Postpone     bool             `flag:"-p"                desc:"Postpone first run until FS event or manual ^R."`
	Extensions   FlagExtensions   `flag:"-e"  init:"go,mod" desc:"Extensions to watch; multi."`
	Watch        FlagWatch        `flag:"-w"  init:"."      desc:"Paths to watch, relative to CWD; multi."`
	IgnoredPaths FlagIgnoredPaths `flag:"-i"                desc:"Ignored paths, relative to CWD; multi."`
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

Hotkeys / control codes:

	3     ^C          Kill subprocess with SIGINT. Repeat within 1s to kill gow.
	18    ^R          Kill subprocess with SIGTERM, restart.
	20    ^T          Kill subprocess with SIGTERM. Repeat within 1s to kill gow.
	28    ^\          Kill subprocess with SIGQUIT. Repeat within 1s to kill gow.
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

func (self Opt) LogSubErr(err error) {
	if err == nil {
		if self.Verb {
			log.Println(`exit ok`)
		}
		return
	}

	if self.Verb || !self.ShouldSkipErr(err) {
		log.Println(`subcommand error:`, err)
	}
}

/**
`go run` reports exit code to stderr. `go test` reports test failures.
In those cases, we suppress the "exit code" error to avoid redundancy.
*/
func (self Opt) ShouldSkipErr(err error) bool {
	head := gg.Head(self.Args)
	return (head == `run` || head == `test`) && e.As(err, new(*exec.ExitError))
}

func (self Opt) TermPre() { self.Pre.Dump(log.Writer()) }

func (self Opt) TermSuf() { self.Suf.Dump(log.Writer()) }

func (self Opt) TermInter() {
	self.TermPre()
	self.TermClear()
}

func (self Opt) TermClear() {
	if self.ClearHard {
		gg.Write(os.Stdout, TermEscCup+TermEscClearHard)
	} else if self.ClearSoft {
		gg.Write(os.Stdout, TermEscCup+TermEscClearSoft)
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

func (self Opt) AllowPath(path string) bool {
	return self.IgnoredPaths.Allow(path) && self.Extensions.Allow(path)
}
