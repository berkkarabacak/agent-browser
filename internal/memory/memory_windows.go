//go:build windows

package memory

import (
	"fmt"
	"syscall"
	"unsafe"
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// SystemMemory reads installed and available physical RAM.
func SystemMemory() (Info, error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel.NewProc("GlobalMemoryStatusEx")
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	r, _, err := proc.Call(uintptr(unsafe.Pointer(&ms)))
	if r == 0 {
		if err != nil {
			return Info{}, err
		}
		return Info{}, fmt.Errorf("GlobalMemoryStatusEx failed")
	}
	return Info{Total: ms.TotalPhys, Free: ms.AvailPhys}, nil
}
