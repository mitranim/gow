//go:build windows

package main

import (
	"os"
	"syscall"
)

/*
Windows lacks SIGHUP and SIGQUIT. We register only SIGINT and SIGTERM, which
are the signals that `signal.Notify` can actually deliver on Windows: SIGINT
is raised by Ctrl-C in the console, and SIGTERM is delivered when an external
process politely asks us to exit.
*/
var (
	KILL_SIGS = []syscall.Signal{syscall.SIGINT, syscall.SIGTERM}

	/*
		The ^\ hotkey would normally raise SIGQUIT. Windows has no equivalent,
		so we map it to SIGTERM, which behaves the same way for our purposes:
		the broadcast handler will terminate descendant processes.
	*/
	sigQuit = syscall.SIGTERM
)

/*
Windows has no `kill` syscall. We use `os.Process.Kill`, which calls
`TerminateProcess`. This is unconditional and immediate: graceful signals
like SIGTERM cannot be delivered to another process on Windows. From the
caller's perspective the descendant process disappears, which is the
behavior we need for restart and shutdown.
*/
func signalProc(pid int, _ syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

/*
On Unix this re-raises the signal so the runtime exits with the conventional
`128 + signal` status. On Windows this distinction does not exist; the
deferred `Main.Exit` will call `os.Exit(0)` in the normal shutdown path.
*/
func signalSelf(_ syscall.Signal) error { return nil }
