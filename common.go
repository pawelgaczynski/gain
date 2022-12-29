package gain

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

type void struct{}

var member void

const uint64Size = 64

func createClientAddr() (uintptr, uint64) {
	var clientLen = new(uint32)
	clientAddr := &unix.RawSockaddrAny{}
	*clientLen = unix.SizeofSockaddrAny
	return uintptr(unsafe.Pointer(clientAddr)), uint64(uintptr(unsafe.Pointer(clientLen)))
}
