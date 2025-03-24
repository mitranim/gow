package main

import (
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/mitranim/gg"
)

// Used by the ^C hotkey.
const DoubleInputDelay = time.Second

/*
Standard input/output adapter for terminal raw mode. Raw mode allows us to
support our own control codes, but we're also responsible for interpreting
common ASCII codes into OS signals, and optionally for echoing other characters
to stdout. This adapter is unnecessary in non-raw mode where we simply pipe
stdio to/from the child process.
*/
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
TODO: ideally, this should be used only in terminal raw mode. See the comment in
`Cmd.MakeCmd` that explains the issue.
*/
func (self *Stdio) Run() {
	self.LastInst = time.Now()

	for {
		var buf [1]byte
		size, err := os.Stdin.Read(buf[:])
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			log.Println(`error when reading stdin, shutting down stdio:`, err)
			return
		}
		gg.Try(err)
		if size <= 0 {
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

	case CODE_PRINT_HELP, CODE_PRINT_HELP_MACOS:
		self.OnCodePrintHelp()

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

// TODO include all current subproces with their args.
func (*Stdio) OnCodePrintCommand() {
	log.Printf(`current command: %q`, os.Args)
}

func (*Stdio) OnCodePrintHelp() { log.Println(HOTKEY_HELP) }

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
	if self.Main().GetEchoMode() == EchoModeGow {
		gg.Nop2(writeByte(os.Stdout, char))
	}
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
