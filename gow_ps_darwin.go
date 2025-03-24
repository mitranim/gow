//go:build darwin

package main

import (
	"os"

	"github.com/mitranim/gg"
	"golang.org/x/sys/unix"
)

func SubPids(topPid int, verb bool) ([]int, error) {
	pid := os.Getpid()
	pids, err := SubPidsViaSyscall(pid)
	if err == nil {
		return pids, nil
	}
	if verb {
		log.Println(`unable to get pids via syscall, falling back on "ps":`, err)
	}
	return SubPidsViaPs(pid)
}

func SubPidsViaSyscall(topPid int) ([]int, error) {
	// Get all processes.
	infos, err := unix.SysctlKinfoProcSlice(`kern.proc.all`)
	if err != nil {
		return nil, gg.Wrap(err, `failed to get process list`)
	}

	// Index child pids by ppid.
	ppidToPids := make(map[int][]int, len(infos))
	for _, info := range infos {
		pid := int(info.Proc.P_pid)
		ppid := int(info.Eproc.Ppid)
		ppidToPids[ppid] = append(ppidToPids[ppid], pid)
	}

	return procIndexToDescs(ppidToPids, topPid, 0), nil
}
