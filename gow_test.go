package main

import (
	"fmt"
	"path/filepath"
	"testing"

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
		`cmd`,
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
		gtest.NoError(tar.Parse(`one,two,three`))
		gtest.Equal(tar, FlagExtensions{`one`, `two`, `three`})
	}

	{
		var tar FlagExtensions
		gtest.NoError(tar.Parse(`one`))
		gtest.NoError(tar.Parse(`two`))
		gtest.NoError(tar.Parse(`three`))
		gtest.Equal(tar, FlagExtensions{`one`, `two`, `three`})
	}
}

func TestOpt_AllowPath(t *testing.T) {
	defer gtest.Catch(t)

	test := func(path, ignore string, exp bool) {
		var opt Opt
		opt.Init([]string{`-i`, ignore, `cmd`})

		gtest.Eq(
			opt.AllowPath(filepath.Join(cwd, path)),
			exp,
			fmt.Sprintf(`path: %q; ignore: %q`, path, ignore),
		)
	}

	test(`file.go`, ``, true)
	test(`to/file.go`, ``, true)
	test(`to/file`, ``, false)
	test(`to/file.txt`, ``, false)
	test(`to/file.go.txt`, ``, false)
	test(`to/file.go`, `to`, false)
	test(`to/file.go`, `yo,to`, false)
	test(`to/file.go`, `yo,./to/`, false)
	test(`to/file.go`, `file`, true)
	test(`to/file.go`, ``, true)
	test(`.hidden/file.go`, ``, true)
	test(`.hidden/ignore/file.go`, `.hidden/ignore`, false)
	test(`.hidden/no/file.go`, `.hidden/ignore`, true)
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
