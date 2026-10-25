//go:build !linux && !windows

package memory

import "fmt"

// SystemMemory is unavailable on this operating system.
func SystemMemory() (Info, error) {
	return Info{}, fmt.Errorf("memory detection is not implemented on this OS")
}
