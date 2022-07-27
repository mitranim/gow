/**
Go Watch: missing watch mode for the "go" command. Invoked exactly like the
"go" command, but also watches Go files and reruns on changes.
*/
package main

import (
	e "errors"
	l "log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/mitranim/gg"
)

var (
	log = l.New(os.Stderr, `[gow] `, 0)
	cwd = gg.Cwd()
)

func main() {
	var main Main
	defer main.Exit()
	defer main.Deinit()
	main.Init()
	main.Run()
}

type Main struct {
	Opt         Opt
	Cmd         Cmd
	TermState   TermState
	Watcher     Watcher
	LastChar    byte
	LastInst    time.Time
	ChanSignals gg.Chan[os.Signal]
	ChanRestart gg.Chan[struct{}]
	ChanKill    gg.Chan[syscall.Signal]
}

func (self *Main) Init() {
	self.Opt.Init(os.Args[1:])

	self.ChanRestart.Init()
	self.ChanKill.Init()

	self.Cmd.Init()
	self.StdinInit()
	self.SigInit()
	self.WatchInit()
	self.TermInit()
}

/**
We MUST call this before exiting because:

	* We modify global OS state: terminal, subprocs.
	* OS will NOT auto-cleanup after us.

Otherwise:

	* Terminal is left in unusable state.
	* Subprocs become orphan daemons.

We MUST call this manually before using `syscall.Kill` or `syscall.Exit` on the
current process. Syscalls terminate the process bypassing Go `defer`.
*/
func (self *Main) Deinit() {
	self.TermDeinit()
	self.WatchDeinit()
	self.SigDeinit()
	self.Cmd.Deinit()
}

func (self *Main) Run() {
	go self.StdinRun()
	go self.SigRun()
	go self.WatchRun()
	self.CmdRun()
}

func (self *Main) TermInit() {
	if self.Opt.Raw {
		self.TermState.Init()
	}
}

func (self *Main) TermDeinit() { self.TermState.Deinit() }

func (self *Main) StdinInit() { self.AfterByte(0) }

/**
See `Main.InitTerm`. "Raw mode" allows us to support our own control codes,
but we're also responsible for interpreting common ASCII codes into OS signals.
*/
func (self *Main) StdinRun() {
	buf := make([]byte, 1, 1)

	for {
		size, err := os.Stdin.Read(buf)
		if err != nil || size == 0 {
			return
		}
		self.OnByte(buf[0])
	}
}

/**
Interpret known ASCII codes as OS signals.
Otherwise forward the input to the subprocess.
*/
func (self *Main) OnByte(val byte) {
	defer recLog()
	defer self.AfterByte(val)

	switch val {
	case CODE_INTERRUPT:
		self.OnCodeInterrupt()

	case CODE_QUIT:
		self.OnCodeQuit()

	case CODE_PRINT_COMMAND:
		self.OnCodePrintCommand()

	case CODE_RESTART:
		self.OnCodeRestart()

	case CODE_STOP:
		self.OnCodeStop()

	default:
		self.OnByteAny(val)
	}
}

func (self *Main) AfterByte(val byte) {
	self.LastChar = val
	self.LastInst = time.Now()
}

func (self *Main) OnCodeInterrupt() {
	self.OnCodeSig(CODE_INTERRUPT, syscall.SIGINT, `^C`)
}

func (self *Main) OnCodeQuit() {
	self.OnCodeSig(CODE_QUIT, syscall.SIGQUIT, `^\`)
}

func (self *Main) OnCodePrintCommand() {
	log.Printf(`current command: %q`, os.Args)
}

func (self *Main) OnCodeRestart() {
	if self.Opt.Verb {
		log.Println(`received ^R, restarting`)
	}
	self.Restart()
}

func (self *Main) OnCodeStop() {
	self.OnCodeSig(CODE_STOP, syscall.SIGTERM, `^T`)
}

func (self *Main) OnByteAny(char byte) { self.Cmd.WriteChar(char) }

func (self *Main) OnCodeSig(code byte, sig syscall.Signal, desc string) {
	if self.IsCodeRepeated(code) {
		log.Printf(`received %[1]v%[1]v, shutting down`, desc)
		self.Kill(sig)
		return
	}

	if self.Opt.Verb {
		log.Printf(`received %[1]v, stopping subprocess`, desc)
	}
	self.Cmd.Broadcast(sig)
}

func (self *Main) IsCodeRepeated(val byte) bool {
	return self.LastChar == val && time.Now().Sub(self.LastInst) < time.Second
}

/**
We override Go's default signal handling to ensure cleanup before exit.
Cleanup is necessary to restore the previous terminal state and kill any
sub-sub-processes.

The set of signals registered here MUST match the set of signals explicitly
handled by this program; see below.
*/
func (self *Main) SigInit() {
	self.ChanSignals.InitCap(1)
	signal.Notify(self.ChanSignals, KILL_SIGS_OS...)
}

func (self *Main) SigDeinit() {
	if self.ChanSignals != nil {
		signal.Stop(self.ChanSignals)
	}
}

func (self *Main) SigRun() {
	for val := range self.ChanSignals {
		// Should work on all Unix systems. At the time of writing,
		// we're not prepared to support other systems.
		sig := val.(syscall.Signal)

		if self.Opt.Verb {
			log.Println(`received signal:`, sig)
		}

		if gg.Has(KILL_SIGS, sig) {
			self.Kill(sig)
		}
	}
}

func (self *Main) WatchInit() {
	wat := new(WatchNotify)
	wat.Init(self)
	self.Watcher = wat
}

func (self *Main) WatchDeinit() {
	if self.Watcher != nil {
		self.Watcher.Deinit()
		self.Watcher = nil
	}
}

func (self *Main) WatchRun() {
	if self.Watcher != nil {
		self.Watcher.Run(self)
	}
}

func (self *Main) CmdRun() {
	for {
		self.Cmd.Restart(self)

		select {
		case <-self.ChanRestart:
			self.Opt.TermClear()
			continue

		case val := <-self.ChanKill:
			self.Cmd.Broadcast(val)
			self.Deinit()
			gg.Nop1(syscall.Kill(os.Getpid(), val))
			return
		}
	}
}

func (self *Main) CmdWait(cmd *exec.Cmd) {
	err := cmd.Wait()

	if err != nil {
		// `go run` reports the program's exit code to stderr.
		// In this case we suppress the error message to avoid redundancy.
		if !(gg.Head(self.Opt.Args) == `run` && e.As(err, new(*exec.ExitError))) {
			log.Println(`subcommand error:`, err)
		}
	} else if self.Opt.Verb {
		log.Println(`exit ok`)
	}

	self.Opt.Sep.Dump(log.Writer())
}

// Must be deferred.
func (self *Main) Exit() {
	err := gg.AnyErrTraced(recover())
	if err != nil {
		self.Opt.LogErr(err)
		os.Exit(1)
	}
	os.Exit(0)
}

func (self *Main) Restart() { self.ChanRestart.SendZeroOpt() }

func (self *Main) Kill(val syscall.Signal) { self.ChanKill.SendOpt(val) }
