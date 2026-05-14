package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mitranim/gg"
)

func main() {
	exe := gg.Try1(os.Executable())
	exe = filepath.Join(filepath.Dir(exe), `proc1.exe`)

	cmd := exec.Command(exe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	gg.Try(cmd.Run())
}
