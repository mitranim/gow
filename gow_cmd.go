package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/mitranim/gg"
)

type Cmd struct {
	Mained
	sync.Mutex
	Cmd   *exec.Cmd
	Stdin io.WriteCloser // Writer half of `self.Cmd.Stdin` for forwarding.
}

func (self *Cmd) Deinit() {
	defer gg.Lock(self).Unlock()
	self.DeinitUnsync()
}

func (self *Cmd) DeinitUnsync() {
	// Should kill the entire sub-process group.
	self.BroadcastUnsync(syscall.SIGTERM)
	self.Cmd = nil
	self.Stdin = nil
}

/*
Note: because the sub-process state changes concurrently, it may change
from "running" to "stopped" immediately after this function returns, while the
caller is taking actions based on the old "running" state. Decision making at
the callsite must account for this.
*/
func (self *Cmd) IsRunning() bool {
	defer gg.Lock(self).Unlock()
	return self.IsRunningUnsync()
}

func (self *Cmd) IsRunningUnsync() bool {
	cmd := self.Cmd
	return cmd != nil && cmd.ProcessState == nil
}

func (self *Cmd) Restart() {
	defer gg.Lock(self).Unlock()
	self.DeinitUnsync()

	cmd, stdin := self.MakeCmd()
	if cmd == nil {
		return
	}

	// Starting the sub-process populates its `.Process`,
	// which allows us to kill the sub-process group on demand.
	err := cmd.Start()
	if err != nil {
		log.Println(`unable to start subcommand:`, err)
		return
	}

	self.Cmd = cmd
	self.Stdin = stdin
	go self.Main().CmdWait(cmd)
}

func (self *Cmd) MakeCmd() (*exec.Cmd, io.WriteCloser) {
	opt := self.Main().Opt
	cmd := exec.Command(opt.Cmd, opt.Args...)

	/**
	Causes the OS to assign process group ID = `cmd.Process.Pid`.
	We use this to broadcast signals to the entire sub-process group.
	Or at least that's how it's supposed to work.
	TODO: use `.Pdeathsig` on Linux.
	*/
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !opt.Raw {
		cmd.Stdin = os.Stdin
		return cmd, nil
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println(`unable to initialize subcommand stdin:`, err)
		return nil, nil
	}
	return cmd, stdin
}

func (self *Cmd) Broadcast(sig syscall.Signal) {
	defer gg.Lock(self).Unlock()
	self.BroadcastUnsync(sig)
}

/*
Sends the signal to the sub-process group, denoted by the negative sign on the
PID. Requires `syscall.SysProcAttr{Setpgid: true}`.
*/
func (self *Cmd) BroadcastUnsync(sig syscall.Signal) {
	proc := self.ProcUnsync()
	if proc == nil {
		return
	}
	gg.Nop1(syscall.Kill(-proc.Pid, sig))
}

func (self *Cmd) WriteChar(char byte) {
	// On the author's system at the time of writing, locking takes under a
	// microsecond and unlocking takes just over two microseconds. Because this
	// is intended for manual user input, this amount of delay per keystroke
	// seems acceptable.
	defer gg.Lock(self).Unlock()

	stdin := self.Stdin
	if stdin == nil {
		return
	}

	_, err := writeByte(stdin, char)
	if errors.Is(err, os.ErrClosed) {
		self.Stdin = nil
		return
	}
	gg.Try(err)
}

func (self *Cmd) ProcUnsync() *os.Process {
	cmd := self.Cmd
	if cmd != nil {
		return cmd.Process
	}
	return nil
}
