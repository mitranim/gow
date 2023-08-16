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
