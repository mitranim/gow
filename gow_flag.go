package main

import (
	"io"
	"strings"

	"github.com/mitranim/gg"
)

type FlagStrMultiline string

func (self *FlagStrMultiline) Parse(src string) error {
	*self += FlagStrMultiline(withNewline(REP_SINGLE_MULTI(src)))
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
	gg.AppendVals(self, vals...)
	return
}

func (self FlagExtensions) Allow(path string) bool {
	return gg.IsEmpty(self) || gg.Has(self, cleanExtension(path))
}

type FlagWatch []string

func (self *FlagWatch) Parse(src string) error {
	gg.AppendVals(self, commaSplit(src)...)
	return nil
}

type FlagIgnoredPaths []string

func (self *FlagIgnoredPaths) Parse(src string) error {
	vals := FlagIgnoredPaths(commaSplit(src))
	vals.Norm()
	gg.AppendVals(self, vals...)
	return nil
}

func (self FlagIgnoredPaths) Norm() {
	gg.MapMut(self, toAbsDirPath)
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
