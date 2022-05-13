package main

import (
	"github.com/rjeczalik/notify"
	"os"
	"path/filepath"
	"testing"
)

type TestFsEvent struct {
	path string
}

func (e *TestFsEvent) Event() notify.Event {
	return 0
}

func (e *TestFsEvent) Path() string {
	return e.path
}

func (e *TestFsEvent) Sys() interface{} {
	return nil
}

func BenchmarkShouldRestart(b *testing.B) {
	EXTENSIONS = &flagStrings{validateExtension, decorateExtension, []string{"ext1", "ext2", "ext3"}}
	IGNORED_PATHS = &flagStrings{validatePath, decorateIgnore, []string{"./ignore1", "ignore2", "ignore3"}}

	cwd, _ := os.Getwd()
	event := &TestFsEvent{path: filepath.Join(cwd, "ignore3/file.ext3")}

	for i := 0; i < b.N; i++ {
		_, _ = shouldRestart(event)
	}
}

func TestShouldRestart(t *testing.T) {

	type shouldRestartCase struct {
		path       string
		ignore     []string
		extensions []string
		expected   bool
	}

	cases := []shouldRestartCase{
		{path: "file.go", extensions: []string{"go"}, ignore: []string{}, expected: true},
		{path: "to/file.go", extensions: []string{"go"}, ignore: []string{}, expected: true},
		{path: "to/file", extensions: []string{"go"}, ignore: []string{}, expected: false},
		{path: "to/file.txt", extensions: []string{"go"}, ignore: []string{}, expected: false},
		{path: "to/file.go.txt", extensions: []string{"go"}, ignore: []string{}, expected: false},
		{path: "to/file.go", extensions: []string{"go"}, ignore: []string{"to"}, expected: false},
		{path: "to/file.go", extensions: []string{"go"}, ignore: []string{"yo", "to"}, expected: false},
		{path: "to/file.go", extensions: []string{"go"}, ignore: []string{"yo", "./to/"}, expected: false},
		{path: "to/file.go", extensions: []string{"go"}, ignore: []string{"file"}, expected: true},
		{path: "to/file.go", extensions: []string{"go"}, ignore: []string{}, expected: true},
		{path: ".hidden/file.go", extensions: []string{"go"}, ignore: []string{}, expected: true},
		{path: ".hidden/ignore/file.go", extensions: []string{"go"}, ignore: []string{".hidden/ignore"}, expected: false},
		{path: ".hidden/no/file.go", extensions: []string{"go"}, ignore: []string{".hidden/ignore"}, expected: true},
	}

	cwd, _ := os.Getwd()

	for _, testCase := range cases {
		EXTENSIONS = &flagStrings{validateExtension, decorateExtension, testCase.extensions}
		IGNORED_PATHS = &flagStrings{validatePath, decorateIgnore, testCase.ignore}

		EXTENSIONS.Prepare()
		IGNORED_PATHS.Prepare()

		should, _ := shouldRestart(&TestFsEvent{path: filepath.Join(cwd, testCase.path)})

		if testCase.expected != should {
			t.Error(testCase.expected, should, testCase)
		}
	}
}
