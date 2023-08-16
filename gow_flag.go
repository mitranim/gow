package main

import (
	"io"
	"strings"

	"github.com/mitranim/gg"
)

type FlagStrMultiline string

func (self *FlagStrMultiline) Parse(src string) error {
	*self += FlagStrMultiline(withNewline(REP_SINGLE_MULTI.Replace(src)))
	return nil
}

func (self FlagStrMultiline) Dump(out io.Writer) {
	if len(self) > 0 && out != nil {
		gg.Nop2(out.Write(gg.ToBytes(self)))
	}
}

type FlagExtensions []string

func (self *FlagExtensions) Parse(src string) (err error) {
	defer gg.Rec(&err)
	vals := commaSplit(src)
	gg.Each(vals, validateExtension)
	gg.Append(self, vals...)
	return
}

func (self FlagExtensions) Allow(path string) bool {
	return gg.IsEmpty(self) || gg.Has(self, cleanExtension(path))
}

type FlagWatchDirs []string

func (self *FlagWatchDirs) Parse(src string) error {
	gg.Append(self, commaSplit(src)...)
	return nil
}

type FlagIgnoreDirs []string

func (self *FlagIgnoreDirs) Parse(src string) error {
	vals := FlagIgnoreDirs(commaSplit(src))
	vals.Norm()
	gg.Append(self, vals...)
	return nil
}

func (self FlagIgnoreDirs) Norm() { gg.MapMut(self, toAbsDirPath) }

func (self FlagIgnoreDirs) Allow(path string) bool { return !self.Ignore(path) }

/*
Assumes that the input is an absolute path.
TODO: also ignore if the directory path is an exact match.
*/
func (self FlagIgnoreDirs) Ignore(path string) bool {
	return gg.Some(self, func(val string) bool {
		return strings.HasPrefix(path, val)
	})
}

const (
	EchoModeNone     EchoMode = 0
	EchoModeGow      EchoMode = 1
	EchoModePreserve EchoMode = 2
)

var EchoModes = []EchoMode{
	EchoModeNone,
	EchoModeGow,
	EchoModePreserve,
}

type EchoMode byte

func (self EchoMode) String() string {
	switch self {
	case EchoModeNone:
		return ``
	case EchoModeGow:
		return `gow`
	case EchoModePreserve:
		return `preserve`
	default:
		panic(self.errInvalid())
	}
}

func (self *EchoMode) Parse(src string) error {
	switch src {
	case ``:
		*self = EchoModeNone
	case `gow`:
		*self = EchoModeGow
	case `preserve`:
		*self = EchoModePreserve
	default:
		return gg.Errf(`unsupported echo mode %q; supported modes: %q`, src, gg.Map(EchoModes, EchoMode.String))
	}
	return nil
}

func (self EchoMode) errInvalid() error {
	return gg.Errf(`invalid echo mode %v; valid modes: %v`, self, EchoModes)
}
