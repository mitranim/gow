package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/mitranim/gg"
)

const (
	// These names reflect standard naming and meaning.
	// See our re-interpretation below.
	ASCII_END_OF_TEXT      = 3  // ^C
	ASCII_FILE_SEPARATOR   = 28 // ^\
	ASCII_DEVICE_CONTROL_2 = 18 // ^R
	ASCII_DEVICE_CONTROL_4 = 20 // ^T
	ASCII_UNIT_SEPARATOR   = 31 // ^- or ^?

	// These names reflect our re-interpretation of standard codes.
	CODE_INTERRUPT     = ASCII_END_OF_TEXT
	CODE_QUIT          = ASCII_FILE_SEPARATOR
	CODE_RESTART       = ASCII_DEVICE_CONTROL_2
	CODE_STOP          = ASCII_DEVICE_CONTROL_4
	CODE_PRINT_COMMAND = ASCII_UNIT_SEPARATOR
)

var (
	FD_TERM      = syscall.Stdin
	KILL_SIGS    = []syscall.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM}
	KILL_SIGS_OS = gg.Map(KILL_SIGS, toOsSignal[syscall.Signal])
	RE_WORD      = regexp.MustCompile(`^\w+$`)
	PATH_SEP     = string([]rune{os.PathSeparator})

	REP_SINGLE_MULTI = strings.NewReplacer(
		`\r\n`, gg.Newline,
		`\r`, gg.Newline,
		`\n`, gg.Newline,
	)
)

/**
Making `.main` private reduces the chance of accidental cyclic walking by
reflection tools such as pretty printers.
*/
type Mained struct{ main *Main }

func (self *Mained) Init(val *Main) { self.main = val }
func (self *Mained) Main() *Main    { return self.main }

/**
Implemented by `notify.EventInfo`.
Path must be an absolute filesystem path.
*/
type FsEvent interface{ Path() string }

// Implemented by `WatchNotify`.
type Watcher interface {
	Init(*Main)
	Deinit()
	Run()
}

func commaSplit(val string) []string {
	if len(val) == 0 {
		return nil
	}
	return strings.Split(val, `,`)
}

func cleanExtension(val string) string {
	ext := filepath.Ext(val)
	if len(ext) > 0 && ext[0] == '.' {
		return ext[1:]
	}
	return ext
}

func validateExtension(val string) {
	if !RE_WORD.MatchString(val) {
		panic(gg.Errf(`invalid extension %q`, val))
	}
}

func toAbsPath(val string) string {
	if !filepath.IsAbs(val) {
		val = filepath.Join(cwd, val)
	}
	return filepath.Clean(val)
}

func toDirPath(val string) string {
	if val == `` || strings.HasSuffix(val, PATH_SEP) {
		return val
	}
	return val + PATH_SEP
}

func toAbsDirPath(val string) string { return toDirPath(toAbsPath(val)) }

func toOsSignal[A os.Signal](src A) os.Signal { return src }

func recLog() {
	val := recover()
	if val != nil {
		log.Println(val)
	}
}

func withNewline[A ~string](val A) A {
	if gg.HasNewlineSuffix(val) {
		return val
	}
	return val + A(gg.Newline)
}
