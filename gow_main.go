/*
Go Watch: missing watch mode for the "go" command. Invoked exactly like the
"go" command, but also watches Go files and reruns on changes.
*/
package main

import (
	l "log"
	"os"
	"syscall"

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
	Stdio       Stdio
	Watcher     Watcher
	Term        Term
	Sig         Sig
	ChanRestart gg.Chan[struct{}]
	ChanKill    gg.Chan[syscall.Signal]
	Pid         int
}

func (self *Main) Init() {
	self.Opt.Init(os.Args[1:])
	self.Term.Init(self)
	self.ChanRestart.Init()
	self.ChanKill.Init()
	self.Cmd.Init(self)
	self.Sig.Init(self)
	self.WatchInit()
	self.Stdio.Init(self)
}

/*
We MUST call this before exiting because:

  - We modify global OS state: terminal, subprocs.
  - OS will NOT auto-cleanup after us.

Otherwise:

  - Terminal is left in unusable state.
  - Subprocs become orphan daemons.

We MUST call this manually before using `syscall.Kill` or `syscall.Exit` on the
current process. Syscalls terminate the process bypassing Go `defer`.
*/
func (self *Main) Deinit() {
	self.Stdio.Deinit()
	self.Term.Deinit()
	self.WatchDeinit()
	self.Sig.Deinit()
	self.Cmd.Deinit()
}

func (self *Main) Run() {
	if self.Term.IsActive() {
		go self.Stdio.Run()
	}
	go self.Sig.Run()
	go self.WatchRun()
	self.CmdRun()
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
		self.Watcher.Run()
	}
}

func (self *Main) CmdRun() {
	if !self.Opt.Postpone {
		self.Cmd.Restart()
	}

	for {
		select {
		case <-self.ChanRestart:
			self.Opt.TermInter()
			self.Cmd.Restart()

		case sig := <-self.ChanKill:
			self.kill(sig)
			return
		}
	}
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

func (self *Main) OnFsEvent(event FsEvent) {
	if !self.ShouldRestart(event) {
		return
	}
	if self.Opt.Verb {
		log.Println(`restarting on FS event:`, event)
	}
	self.Restart()
}

func (self *Main) ShouldRestart(event FsEvent) bool {
	return event != nil &&
		!(self.Opt.Lazy && self.Cmd.IsRunning()) &&
		self.Opt.AllowPath(event.Path())
}

func (self *Main) Restart() { self.ChanRestart.SendZeroOpt() }

func (self *Main) Kill(val syscall.Signal) { self.ChanKill.SendOpt(val) }

// Must be called only on the main goroutine.
func (self *Main) kill(sig syscall.Signal) {
	/**
	This should terminate any descendant processes, using their default behavior
	for the given signal. If any misbehaving processes do not terminate on a
	kill signal, this is out of our hands for now. We could use SIGKILL to
	ensure termination, but it's unclear if we should.
	*/
	self.Cmd.Broadcast(sig)

	/**
	This should restore previous terminal state and un-register our custom signal
	handling.
	*/
	self.Deinit()

	/**
	Re-send the signal after un-registering our signal handling. If our process is
	still running by the time the signal is received, the signal will be handled
	by the Go runtime, using the default behavior. Most of the time, this signal
	should not be received because after calling this method, we also return
	from the main function.
	*/
	gg.Nop1(syscall.Kill(os.Getpid(), sig))
}

func (self *Main) GetEchoMode() EchoMode {
	if self.Term.IsActive() {
		return self.Opt.Echo
	}
	return EchoModeNone
}
