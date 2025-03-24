//go:build dragonfly || freebsd || netbsd || openbsd

package main

// On BSD, we simply fall back on `ps`.
func SubPids(topPid int, _ bool) ([]int, error) {
	return SubPidsViaPs(topPid)
}
