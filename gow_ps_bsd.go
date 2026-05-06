//go:build !(darwin || linux || windows)

package main

/*
On systems where we don't implement a "native" fast PS,
we fall back on shelling out to `ps`.
*/
func SubPids(topPid int, _ bool) ([]int, error) {
	return SubPidsViaPs(topPid)
}
