//go:build linux

package memory

import "os"

// SystemMemory reads MemTotal and MemAvailable from /proc/meminfo.
func SystemMemory() (Info, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	return ParseMeminfo(f)
}
