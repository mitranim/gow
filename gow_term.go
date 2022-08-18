package main

import (
	"github.com/mitranim/gg"
	"golang.org/x/sys/unix"
)

/**
By default, any regular terminal uses what's known as "cooked mode". It buffers
lines before sending them to the foreground process, and interprets some ASCII
control codes on stdin by sending the corresponding OS signals to the process.
We switch it into "raw mode", where it immediately forwards inputs to our
process's stdin, and doesn't interpret special ASCII codes. This allows to
support special key combinations such as ^R for restarting a subprocess.

The terminal state is shared between all super- and sub-processes. Changes
persist even after our process terminates. We endeavor to restore the previous
state before exiting.

References:

	https://en.wikibooks.org/wiki/Serial_Programming/termios

	man termios
*/
type TermState struct{ gg.Opt[unix.Termios] }

func (self *TermState) Init(main *Main) {
	self.Deinit()

	if !main.Opt.Raw {
		return
	}

	state, err := unix.IoctlGetTermios(FD_TERM, ioctlReadTermios)
	if err != nil {
		log.Println(`unable to read terminal state:`, err)
		return
	}
	prev := *state

	/**
	Don't echo stdin to stdout. Most terminals, in addition to echoing non-special
	characters, also have special support for various ASCII control codes. Codes
	that send signals are cosmetically printed as hotkeys such as `^C`, `^R`,
	and so on. The delete code (127) should cause the terminal to delete one
	character before the caret, moving the caret. At the time of writing, the
	built-in MacOS terminal doesn't properly echo characters when operating in
	raw mode. For example, the delete code is printed back as `^?`, which is
	rather jarring. As a workaround, we suppress default echoing in raw mode,
	and do it ourselves in the `Stdio` type.
	*/
	state.Lflag &^= unix.ECHO

	// Don't buffer lines.
	state.Lflag &^= unix.ICANON

	// No signals.
	state.Lflag &^= unix.ISIG

	// Seems unnecessary on my system. Might be needed elsewhere.
	// state.Cflag |= unix.CS8
	// state.Cc[unix.VMIN] = 1
	// state.Cc[unix.VTIME] = 0

	err = unix.IoctlSetTermios(FD_TERM, ioctlWriteTermios, state)
	if err != nil {
		log.Println(`unable to switch terminal to raw mode:`, err)
		return
	}

	self.Set(prev)
}

func (self *TermState) Deinit() {
	if !self.IsNull() {
		defer self.Clear()
		gg.Nop1(unix.IoctlSetTermios(FD_TERM, ioctlWriteTermios, &self.Val))
	}
}
