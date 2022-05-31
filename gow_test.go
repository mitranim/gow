package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mitranim/gg/gtest"
)

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

type TestFsEvent string

func (self TestFsEvent) Path() string { return string(self) }

func TestOpt_ShouldRestart(t *testing.T) {
	defer gtest.Catch(t)

	test := func(path, ignore string, exp bool) {
		var opt Opt
		opt.Init([]string{`-i`, ignore, `cmd`})

		gtest.Eq(
			opt.ShouldRestart(TestFsEvent(filepath.Join(cwd, path))),
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

func BenchmarkOpt_ShouldRestart(b *testing.B) {
	event := FsEvent(TestFsEvent(filepath.Join(cwd, `ignore3/file.ext3`)))

	var opt Opt
	opt.Init([]string{
		`-e=ext1`,
		`-e=ext2`,
		`-e=ext3`,
		`-i=./ignore1`,
		`-i=ignore2`,
		`-i=ignore3`,
		`cmd`,
	})
	gtest.False(opt.ShouldRestart(event))

	b.ResetTimer()

	for ind := 0; ind < b.N; ind++ {
		opt.ShouldRestart(event)
	}
}
