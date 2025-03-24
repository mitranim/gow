package main

import (
	"os/exec"
	"regexp"
	"strconv"

	"github.com/mitranim/gg"
)

func SubPidsViaPs(topPid int) ([]int, error) {
	cmd := exec.Command(`ps`, `-eo`, `pid=,ppid=`)
	var buf gg.Buf
	cmd.Stdout = &buf

	err := cmd.Run()
	if err != nil {
		return nil, gg.Wrap(err, `unexpected error: unable to invoke "ps" to get subproc pids`)
	}

	var psPid int
	if cmd.Process != nil {
		psPid = cmd.Process.Pid
	}
	return PsOutToSubPids(buf.String(), topPid, psPid), nil
}

func PsOutToSubPids(src string, topPid, skipPid int) []int {
	return procIndexToDescs(procPairsToIndex(psOutToProcPairs(src)), topPid, skipPid)
}

/*
Takes an index that maps ppids to child pids, and returns a slice of all
descendant pids of the given ppid, skipping `skipPid`, sorted descending.
*/
func procIndexToDescs(src map[int][]int, topPid, skipPid int) []int {
	found := gg.Set[int]{}
	for _, val := range src[topPid] {
		if val != skipPid {
			addProcDescs(src, val, found)
		}
	}
	return sortPids(found)
}

func sortPids(src gg.Set[int]) []int {
	out := gg.MapKeys(src)
	gg.SortPrim(out)
	gg.Reverse(out)
	return out
}

func addProcDescs(src map[int][]int, ppid int, out gg.Set[int]) {
	if out.Has(ppid) {
		return
	}
	out.Add(ppid)
	for _, val := range src[ppid] {
		addProcDescs(src, val, out)
	}
}

func procPairsToIndex(src [][2]int) map[int][]int {
	out := map[int][]int{}
	for _, val := range src {
		out[val[1]] = append(out[val[1]], val[0])
	}
	return out
}

func psOutToProcPairs(src string) [][2]int {
	return gg.MapCompact(gg.SplitLines(src), lineToProcPair)
}

func lineToProcPair(src string) (_ [2]int) {
	out := rePsLine.FindStringSubmatch(src)
	if len(out) == 3 {
		return [2]int{parsePid(out[1]), parsePid(out[2])}
	}
	return
}

var rePsLine = regexp.MustCompile(`^\s*(-?\d+)\s+(-?\d+)\s*$`)

func parsePid(src string) int {
	return int(gg.Try1(strconv.ParseInt(src, 10, 32)))
}
