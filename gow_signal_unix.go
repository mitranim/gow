//go:build !windows

package main

import (
	"os"
	"syscall"
)

var (
	KILL_SIGS = []syscall.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM}
	sigQuit   = syscall.SIGQUIT
)

func signalProc(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func signalSelf(sig syscall.Signal) error {
	return syscall.Kill(os.Getpid(), sig)
}
