package main

import (
	"strings"

	"github.com/mitranim/gg"
)

type FlagStrMultiline string

func (self *FlagStrMultiline) String() string {
	return strings.ReplaceAll(string(gg.Deref(self)), gg.Newline, `\n`)
}

func (self *FlagStrMultiline) Set(src string) error {
	src = strings.ReplaceAll(src, `\n`, gg.Newline)
	if len(src) > 0 && !gg.HasNewlineSuffix(src) {
		src += gg.Newline
	}

	*self = FlagStrMultiline(src)
	return nil
}

type FlagExtensions []string

func (self *FlagExtensions) Default() {
	gg.AppendVals(self, `go`, `mod`)
}

func (self *FlagExtensions) String() string {
	return commaJoin(gg.Deref(self))
}

func (self *FlagExtensions) Set(src string) (err error) {
	defer gg.Rec(&err)
	*self = commaSplit(src)
	gg.Each(*self, validateExtension)
	return
}

func (self FlagExtensions) Allow(path string) bool {
	return gg.IsEmpty(self) || gg.Has(self, cleanExtension(path))
}

type FlagIgnoredPaths []string

func (self *FlagIgnoredPaths) String() string {
	return commaJoin(gg.Deref(self))
}

func (self *FlagIgnoredPaths) Set(src string) error {
	*self = commaSplit(src)
	self.Norm()
	return nil
}

func (self FlagIgnoredPaths) Norm() {
	for ind := range self {
		self[ind] = toDirPath(toAbsPath(self[ind]))
	}
}

func (self FlagIgnoredPaths) Allow(path string) bool {
	return !self.Ignore(path)
}

// Assumes that the input is an absolute path.
func (self FlagIgnoredPaths) Ignore(path string) bool {
	return gg.Some(self, func(val string) bool {
		return strings.HasPrefix(path, val)
	})
}

type FlagWatch []string

func (self *FlagWatch) Default() { gg.AppendVals(self, `.`) }

func (self *FlagWatch) String() string {
	return commaJoin(gg.Deref(self))
}

func (self *FlagWatch) Set(src string) error {
	*self = commaSplit(src)
	return nil
}
