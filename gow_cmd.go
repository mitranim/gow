package main

import (
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mitranim/gg"
)

type Cmd struct {
	Mained
	Count atomic.Int64
}

func (self *Cmd) Deinit() {
	if self.Count.Load() > 0 {
		self.Broadcast(syscall.SIGTERM)
	}
}

/*
Note: proc count may change immediately after the call. Decision making at the
callsite must account for this.
*/
func (self *Cmd) IsRunning() bool { return self.Count.Load() == 0 }

func (self *Cmd) Restart() {
	self.Deinit()

	main := self.Main()
	opt := main.Opt
	cmd := exec.Command(opt.Cmd, opt.Args...)

	if !main.Term.IsActive() {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Println(`unable to start subcommand:`, err)
		return
	}

	self.Count.Add(1)
	go self.ReportCmd(cmd, time.Now())
}

func (self *Cmd) ReportCmd(cmd *exec.Cmd, start time.Time) {
	defer self.Count.Add(-1)
	opt := self.Main().Opt
	opt.LogCmdExit(cmd.Wait(), time.Since(start))
	opt.TermSuf()
}

/*
Sends the signal to all subprocesses (descendants included).

Worth mentioning: across all the various Go versions tested (1.11 to 1.24), it
seemed that the `go` commands such as `go run` or `go test` do not forward any
interrupt or kill signals to its subprocess, and neither does `go test`. For
us, this means that terminating the immediate child process is worth very
little; we're concerned with terminating the grand-child processes, which may
be spawned by the common cases `go run`, `go test`, or any replacement commands
from `Opt.Cmd`.

In the past, we used subprocess groups for broadcasts. When spawning the child
process, we used `&syscall.SysProcAttr{Setpgid: true}`, and when broadcasting
a signal, we would send it to `-proc.Pid`. Which did seem to work for killing
descendant processes. But creating a subprocess group interferes with stdio and
TTY detection in descendant processes, so we had to give it up, replacing with
the solution below.
*/
func (self *Cmd) Broadcast(sig syscall.Signal) {
	verb := self.Main().Opt.Verb
	pids, err := SubPids(os.Getpid(), verb)
	if err != nil {
		log.Println(err)
		return
	}
	if gg.IsEmpty(pids) {
		return
	}

	if !verb {
		for _, pid := range pids {
			gg.Nop1(syscall.Kill(pid, sig))
			return
		}
	}

	var sent []int
	var unsent []int
	var errs []error

	for _, pid := range pids {
		err := syscall.Kill(pid, sig)
		if err != nil {
			unsent = append(unsent, pid)
			errs = append(errs, err)
		} else {
			sent = append(sent, pid)
		}
	}

	if gg.IsEmpty(errs) {
		log.Printf(
			`sent signal %q to %v subprocesses, pids: %v`,
			sig, len(pids), sent,
		)
	} else {
		log.Printf(
			`tried to send signal %q to %v subprocesses, sent to pids: %v, not sent to pids: %v, errors: %q`,
			sig, len(pids), sent, unsent, errs,
		)
	}
}
