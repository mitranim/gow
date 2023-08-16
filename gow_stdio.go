package main

import (
	"os"
	"syscall"
	"time"

	"github.com/mitranim/gg"
)

const DoubleInputDelay = time.Second

type Stdio struct {
	Mained
	LastChar byte
	LastInst time.Time
}

/*
Doesn't require special cleanup before stopping `gow`. We run only one stdio
loop, without ever replacing it.
*/
func (*Stdio) Deinit() {}

/*
See `(*TermState).Init`. Terminal raw mode allows us to support our own control
codes, but we're also responsible for interpreting common ASCII codes into OS
signals and for echoing other characters to stdout.
*/
func (self *Stdio) Run() {
	if !self.Main().Opt.Raw {
		return
	}

	self.LastInst = time.Now()

	for {
		var buf [1]byte
		size, err := os.Stdin.Read(buf[:])
		if err != nil || size == 0 {
			return
		}
		self.OnByte(buf[0])
	}
}

/*
Interpret known ASCII codes as OS signals.
Otherwise forward the input to the subprocess.
*/
func (self *Stdio) OnByte(char byte) {
	defer recLog()
	defer self.AfterByte(char)

	switch char {
	case CODE_INTERRUPT:
		self.OnCodeInterrupt()

	case CODE_QUIT:
		self.OnCodeQuit()

	case CODE_PRINT_COMMAND:
		self.OnCodePrintCommand()

	case CODE_RESTART:
		self.OnCodeRestart()

	case CODE_STOP:
		self.OnCodeStop()

	default:
		self.OnByteAny(char)
	}
}

func (self *Stdio) AfterByte(char byte) {
	self.LastChar = char
	self.LastInst = time.Now()
}

func (self *Stdio) OnCodeInterrupt() {
	self.OnCodeSig(CODE_INTERRUPT, syscall.SIGINT, `^C`)
}

func (self *Stdio) OnCodeQuit() {
	self.OnCodeSig(CODE_QUIT, syscall.SIGQUIT, `^\`)
}

func (self *Stdio) OnCodePrintCommand() {
	log.Printf(`current command: %q`, os.Args)
}

func (self *Stdio) OnCodeRestart() {
	main := self.Main()
	if main.Opt.Verb {
		log.Println(`received ^R, restarting`)
	}
	main.Restart()
}

func (self *Stdio) OnCodeStop() {
	self.OnCodeSig(CODE_STOP, syscall.SIGTERM, `^T`)
}

func (self *Stdio) OnByteAny(char byte) {
	main := self.Main()
	main.Cmd.WriteChar(char)

	if main.Opt.Echo == EchoModeGow {
		self.WriteChar(char)
	}
}

func (self *Stdio) WriteChar(char byte) {
	buf := [1]byte{char}
	gg.Nop2(os.Stdout.Write(buf[:]))
}

func (self *Stdio) OnCodeSig(code byte, sig syscall.Signal, desc string) {
	main := self.Main()

	if self.IsCodeRepeated(code) {
		log.Println(`received ` + desc + desc + `, shutting down`)
		main.Kill(sig)
		return
	}

	if main.Opt.Verb {
		log.Println(`broadcasting ` + desc + ` to subprocesses; repeat within ` + DoubleInputDelay.String() + ` to kill gow`)
	}
	main.Cmd.Broadcast(sig)
}

func (self *Stdio) IsCodeRepeated(char byte) bool {
	return self.LastChar == char && time.Since(self.LastInst) < DoubleInputDelay
}
