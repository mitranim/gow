//go:build windows

package main

import (
	"errors"
	"os"
	"unsafe"

	"github.com/mitranim/gg"
	"golang.org/x/sys/windows"
)

/*
Windows has no `/proc` and no portable `ps` (the binary that ships with Git
Bash reports MSYS pids, not native Windows pids, so it cannot be matched
against pids from `os.Getpid` or `exec.Cmd.Process.Pid`).

Instead we walk the system-wide process list with `CreateToolhelp32Snapshot`.
This is the same mechanism `tasklist` and Process Explorer use; it returns
native pids and parent pids, which is what `procIndexToDescs` needs to
walk the descendant tree.
*/
func SubPids(_ int, _ bool) ([]int, error) {
	return SubPidsViaSnapshot(os.Getpid())
}

func SubPidsViaSnapshot(topPid int) ([]int, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, gg.Wrap(err, `failed to create process snapshot`)
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	err = windows.Process32First(snap, &entry)
	if err != nil {
		return nil, gg.Wrap(err, `failed to read first process from snapshot`)
	}

	ppidToPids := map[int][]int{}
	for {
		ppidToPids[int(entry.ParentProcessID)] = append(
			ppidToPids[int(entry.ParentProcessID)], int(entry.ProcessID),
		)
		err = windows.Process32Next(snap, &entry)
		if err != nil {
			// `ERROR_NO_MORE_FILES` ends iteration; any other error is fatal.
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, gg.Wrap(err, `failed to advance process snapshot`)
		}
	}

	return procIndexToDescs(ppidToPids, topPid, 0), nil
}
