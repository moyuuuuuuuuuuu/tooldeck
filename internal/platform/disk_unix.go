//go:build darwin || linux

package platform

import "syscall"

func diskCapacity(path string) (uint64, uint64, error) {
	var st syscall.Statfs_t
	err := syscall.Statfs(path, &st)
	return st.Blocks * uint64(st.Bsize), st.Bavail * uint64(st.Bsize), err
}
