//go:build windows

package main

import (
	"os"

	"github.com/mitranim/gg"
	"golang.org/x/term"
)

/*
Windows has no termios. We use `golang.org/x/term`, which configures the
console via `SetConsoleMode` under the hood: `term.MakeRaw` clears
`ENABLE_LINE_INPUT`, `ENABLE_ECHO_INPUT`, and `ENABLE_PROCESSED_INPUT`,
giving us the same byte-at-a-time stdin behavior we get from termios raw
mode on Unix.

The trade-off compared to the Unix path: `term.MakeRaw` always disables
echo, so `EchoModePreserve` is not honored on Windows. `EchoModeNone` and
`EchoModeGow` (the default) work as designed.
*/
type Term struct{ State *term.State }

func (self Term) IsActive() bool { return self.State != nil }

func (self *Term) Deinit() {
	state := self.State
	if state == nil {
		return
	}
	self.State = nil
	gg.Nop1(term.Restore(int(os.Stdin.Fd()), state))
}

func (self *Term) Init(main *Main) {
	self.Deinit()
	if !main.Opt.Raw {
		return
	}

	prev, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Println(`unable to switch terminal to raw mode:`, err)
		return
	}

	if main.Opt.Echo == EchoModePreserve && main.Opt.Verb {
		log.Println(`echo mode "preserve" is not supported on Windows; echo will be disabled`)
	}

	self.State = prev
}
