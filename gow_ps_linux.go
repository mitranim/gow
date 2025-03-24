//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mitranim/gg"
)

func SubPids(topPid int, verb bool) ([]int, error) {
	pid := os.Getpid()
	pids, err := SubPidsViaProcDir(pid)
	if err == nil {
		return pids, nil
	}
	if verb {
		log.Println(`unable to get pids from "/proc", falling back on "ps":`, err)
	}
	return SubPidsViaPs(pid)
}

func SubPidsViaProcDir(topPid int) ([]int, error) {
	procEntries, err := os.ReadDir(`/proc`)
	if err != nil {
		return nil, gg.Wrap(err, `unable to read directory "/proc"`)
	}

	// Index of child pids by ppid.
	ppidToPids := map[int][]int{}

	for _, entry := range procEntries {
		if !entry.IsDir() {
			continue
		}

		pidStr := entry.Name()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			// Non-numeric names don't describe processes, skip.
			continue
		}

		status, err := os.ReadFile(filepath.Join(`/proc`, pidStr, `status`))
		if err != nil {
			// Process may have terminated, skip.
			continue
		}

		ppid := statusToPpid(gg.ToString(status))
		if ppid != 0 {
			ppidToPids[ppid] = append(ppidToPids[ppid], pid)
		}
	}

	return procIndexToDescs(ppidToPids, topPid, 0), nil
}

func statusToPpid(src string) (_ int) {
	const prefix0 = `PPid:`
	const prefix1 = `Ppid:`

	ind := strings.Index(src, prefix0)
	if ind >= 0 {
		ind += len(prefix0)
	} else {
		ind = strings.Index(src, prefix1)
		if ind < 0 {
			return
		}
		ind += len(prefix1)
	}

	src = src[ind:]
	ind = strings.Index(src, "\n")
	if ind < 0 {
		return
	}
	src = src[:ind]
	src = strings.TrimSpace(src)

	out, _ := strconv.Atoi(src)
	return out
}
