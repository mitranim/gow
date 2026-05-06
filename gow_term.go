//go:build !windows

package main

import (
	"syscall"

	"github.com/mitranim/gg"
	"golang.org/x/sys/unix"
)

var FD_TERM = syscall.Stdin

/*
By default, any TTY uses what's known as "cooked mode", where it buffers lines
before sending them to the foreground process, and interprets ASCII control
codes on stdin by sending the corresponding OS signals to the process.

We switch the TTY into "raw mode", where it mostly forwards inputs to our own
process's stdin as-is, and interprets fewer special ASCII codes. This allows to
support special key combinations such as ^R for restarting a subprocess.
Unfortunately, this also makes us responsible for interpreting the rest of the
ASCII control codes. It's possible that our support for those is incomplete.

The TTY state is shared between all super- and sub-processes. Changes persist
even after our process terminates. We endeavor to restore the previous state
before exiting.

References:

	https://en.wikibooks.org/wiki/Serial_Programming/termios

	man termios
*/
type Term struct{ State *unix.Termios }

func (self Term) IsActive() bool { return self.State != nil }

func (self *Term) Deinit() {
	state := self.State
	if state == nil {
		return
	}
	self.State = nil
	gg.Nop1(unix.IoctlSetTermios(FD_TERM, ioctlWriteTermios, state))
}

/*
Goal:

  - Get old TTY state.
  - Set new TTY state.
  - Remember old TTY state to restore it when exiting.

Known issue: due to race conditions, this is incompatible with running multiple
concurrent `gow` processes in the same TTY. Our own `makefile` provides an
example of how to avoid using multiple raw modes concurrently.
*/
func (self *Term) Init(main *Main) {
	self.Deinit()
	if !main.Opt.Raw {
		return
	}

	prev, err := unix.IoctlGetTermios(FD_TERM, ioctlReadTermios)
	if err != nil {
		log.Println(`unable to read terminal state:`, err)
		return
	}
	next := *prev

	/**
	In raw mode, we support multiple modes of echoing stdin to stdout. Each
	approach has different issues.

	Most terminals, in addition to echoing non-special characters, also have
	special support for various ASCII control codes, printing them in the
	so-called "caret notation". Codes that send signals are cosmetically printed
	as hotkeys such as `^C`, `^R`, and so on. The delete code (127) should cause
	the terminal to delete one character before the caret, moving the caret. At
	the time of writing, the built-in MacOS terminal doesn't properly handle the
	delete character when operating in raw mode, printing it in the caret
	notation `^?`, which is a jarring and useless change from non-raw mode.

	The workaround we use by default (mode `EchoModeGow`) is to suppress default
	echoing in raw mode, and echo by ourselves in the `Stdio` type. We don't
	print the caret notation at all. This works fine for most characters, but at
	least in some terminals, deletion via the delete character (see above)
	doesn't seem to work when we echo the character as-is.

	Other modes allow to suppress echoing completely or fall back on the buggy
	terminal default.
	*/
	switch main.Opt.Echo {
	case EchoModeNone:
		next.Lflag &^= unix.ECHO

	case EchoModeGow:
		// We suppress the default echoing here and replicate it ourselves in
		// `Stdio.OnByteAny`.
		next.Lflag &^= unix.ECHO

	case EchoModePreserve:
		// The point of this mode is to preserve the previous echo mode of the
		// terminal, whatever it was.

	default:
		panic(main.Opt.Echo.errInvalid())
	}

	// Don't buffer lines.
	next.Lflag &^= unix.ICANON

	// No signals.
	next.Lflag &^= unix.ISIG

	// Seems unnecessary on my system. Might be needed elsewhere.
	// next.Cflag |= unix.CS8
	// next.Cc[unix.VMIN] = 1
	// next.Cc[unix.VTIME] = 0

	err = unix.IoctlSetTermios(FD_TERM, ioctlWriteTermios, &next)
	if err != nil {
		log.Println(`unable to switch terminal to raw mode:`, err)
		return
	}

	self.State = prev
}
