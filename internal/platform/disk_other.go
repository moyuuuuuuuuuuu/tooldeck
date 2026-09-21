//go:build !darwin && !linux

package platform

import "errors"

func diskCapacity(string) (uint64, uint64, error) {
	return 0, 0, errors.New("disk capacity unavailable on this platform")
}
