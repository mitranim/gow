//go:build !(darwin || dragonfly || freebsd || netbsd || openbsd || windows)

package main

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TCGETS
const ioctlWriteTermios = unix.TCSETS
