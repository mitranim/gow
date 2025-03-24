package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mitranim/gg"
	"github.com/mitranim/gg/gtest"
)

var testIgnoredPath = filepath.Join(cwd, `ignore3/file.ext3`)

var testIgnoredEvent = FsEvent(TestFsEvent(testIgnoredPath))

var testOpt = func() (tar Opt) {
	tar.Init([]string{
		`-e=ext1`,
		`-e=ext2`,
		`-e=ext3`,
		`-i=./ignore1`,
		`-i=ignore2`,
		`-i=ignore3`,
		`some_command`,
	})
	return
}()

var testMain = Main{Opt: testOpt}

type TestFsEvent string

func (self TestFsEvent) Path() string { return string(self) }

func TestFlagExtensions(t *testing.T) {
	defer gtest.Catch(t)

	opt := OptDefault()
	gtest.Equal(opt.Extensions, FlagExtensions{`go`, `mod`})

	{
		var tar FlagExtensions
		gtest.NoErr(tar.Parse(`one,two,three`))
		gtest.Equal(tar, FlagExtensions{`one`, `two`, `three`})
	}

	{
		var tar FlagExtensions
		gtest.NoErr(tar.Parse(`one`))
		gtest.NoErr(tar.Parse(`two`))
		gtest.NoErr(tar.Parse(`three`))
		gtest.Equal(tar, FlagExtensions{`one`, `two`, `three`})
	}
}

func testIgnore[Ignore interface {
	~[]string
	Norm()
	Ignore(string) bool
}](path string, ignore Ignore, exp bool) {
	// Even though we invoke this on a value type, this works because the method
	// mutates the underlying array, not the slice header itself.
	ignore.Norm()
	path = filepath.Join(cwd, path)
	msg := fmt.Sprintf(`ignore: %q; path: %q`, ignore, path)

	if exp {
		gtest.True(ignore.Ignore(path), msg)
	} else {
		gtest.False(ignore.Ignore(path), msg)
	}
}

func TestFlagIgnoreDirs_Ignore(t *testing.T) {
	defer gtest.Catch(t)

	type Ignore = FlagIgnoreDirs

	{
		testIgnore(`one.go`, Ignore{}, false)
		testIgnore(`one.go`, Ignore{`.`}, true)
		testIgnore(`one.go`, Ignore{`*`}, false)
		testIgnore(`one.go`, Ignore{`.`, `two`}, true)
		testIgnore(`one.go`, Ignore{`*`, `two`}, false)
		testIgnore(`one.go`, Ignore{`./*`}, false)
		testIgnore(`one.go`, Ignore{`two`}, false)
		testIgnore(`one.go`, Ignore{`one.go`}, false)
	}

	{
		testIgnore(`one/two.go`, Ignore{}, false)
		testIgnore(`one/two.go`, Ignore{`one/two.go`}, false)

		testIgnore(`one/two.go`, Ignore{`.`}, true)
		testIgnore(`one/two.go`, Ignore{`./.`}, true)
		testIgnore(`one/two.go`, Ignore{`././.`}, true)

		testIgnore(`one/two.go`, Ignore{`*`}, false)
		testIgnore(`one/two.go`, Ignore{`./*`}, false)
		testIgnore(`one/two.go`, Ignore{`*/*`}, false)
		testIgnore(`one/two.go`, Ignore{`./*/*`}, false)

		testIgnore(`one/two.go`, Ignore{`one`}, true)
		testIgnore(`one/two.go`, Ignore{`./one`}, true)

		testIgnore(`one/two.go`, Ignore{`one/.`}, true)
		testIgnore(`one/two.go`, Ignore{`./one/.`}, true)

		testIgnore(`one/two.go`, Ignore{`one/*`}, false)
		testIgnore(`one/two.go`, Ignore{`./one/*`}, false)

		testIgnore(`one/two.go`, Ignore{`one/two`}, false)
		testIgnore(`one/two.go`, Ignore{`./one/two`}, false)

		testIgnore(`one/two.go`, Ignore{`one/two/.`}, false)
		testIgnore(`one/two.go`, Ignore{`./one/two/.`}, false)

		testIgnore(`one/two.go`, Ignore{`one/two/*`}, false)
		testIgnore(`one/two.go`, Ignore{`./one/two/*`}, false)

		testIgnore(`one/two.go`, Ignore{`two`}, false)
		testIgnore(`one/two.go`, Ignore{`one`, `three`}, true)
		testIgnore(`one/two.go`, Ignore{`three`, `one`}, true)
	}

	{
		testIgnore(`.one/two.go`, Ignore{}, false)
		testIgnore(`.one/two.go`, Ignore{`.one`}, true)
		testIgnore(`.one/two.go`, Ignore{`./.one`}, true)
		testIgnore(`.one/two.go`, Ignore{`.one/.`}, true)
		testIgnore(`.one/two.go`, Ignore{`.one/*`}, false)
		testIgnore(`.one/two.go`, Ignore{`three`}, false)
	}

	{
		testIgnore(`.one/two/three.go`, Ignore{`.one`}, true)
		testIgnore(`.one/two/three.go`, Ignore{`.one/.`}, true)
		testIgnore(`.one/two/three.go`, Ignore{`.one/*`}, false)
		testIgnore(`.one/two/three.go`, Ignore{`.one/two`}, true)
		testIgnore(`.one/two/three.go`, Ignore{`.one/two/.`}, true)
		testIgnore(`.one/two/three.go`, Ignore{`.one/two/*`}, false)
		testIgnore(`.one/two/three.go`, Ignore{`.one/three`}, false)
	}
}

func BenchmarkOpt_AllowPath(b *testing.B) {
	gtest.False(testOpt.AllowPath(testIgnoredPath))
	b.ResetTimer()

	for ind := 0; ind < b.N; ind++ {
		testOpt.AllowPath(testIgnoredPath)
	}
}

func BenchmarkMain_ShouldRestart(b *testing.B) {
	gtest.False(testMain.ShouldRestart(testIgnoredEvent))
	b.ResetTimer()

	for ind := 0; ind < b.N; ind++ {
		testMain.ShouldRestart(testIgnoredEvent)
	}
}

func Test_PsOutToSubPids(t *testing.T) {
	defer gtest.Catch(t)

	const SRC = `
  PID  PPID
    1     0
   83     1
   85     1
   91     1
 1634     1
 1909     1
 1951  3967
 1982  3967
  PID  PPID
 2764     1
 3967     1
 3971     1
 3975  3967
 3976  3967
 3977  3967
 4000  3967
 4008  3967
 4009  3967
 4060     1
 4098     1
 4801  4008
 5125     1
 5627     1
 5682  4009
 5683     1
 10 20 30 40 (should be ignored)
`

	ppidToPids := procPairsToIndex(psOutToProcPairs(SRC))

	gtest.Equal(ppidToPids, map[int][]int{
		0:    {1},
		1:    {83, 85, 91, 1634, 1909, 2764, 3967, 3971, 4060, 4098, 5125, 5627, 5683},
		3967: {1951, 1982, 3975, 3976, 3977, 4000, 4008, 4009},
		4008: {4801},
		4009: {5682},
	})

	const topPid = 3967
	const skipPid = 4008

	descs := procIndexToDescs(ppidToPids, topPid, skipPid)
	gtest.Equal(PsOutToSubPids(SRC, topPid, skipPid), descs)

	gtest.Equal(
		descs,
		[]int{5682, 4009, 4000, 3977, 3976, 3975, 1982, 1951},
	)
}

func TestSubPids(t *testing.T) {
	defer gtest.Catch(t)

	/**
	Our process doesn't have any children, so we have to spawn one
	for testing purposes.
	*/

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, `sleep`, `1`)
	cmd.Start()

	pids := gg.Try1(SubPids(os.Getpid(), true))
	gtest.Len(pids, 1)
}
