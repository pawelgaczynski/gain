// Copyright (c) 2023 Paweł Gaczyński
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package virtualmem

import (
	"fmt"
	"log"
	"math"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

var pageSize = os.Getpagesize()

func doubleSize(size int) int {
	return size * 2 //nolint:gomnd // skip gomnd linter since this method is self-documented
}

type VirtualMem struct {
	Buf  []byte
	Size int
}

func (m *VirtualMem) Zeroes() {
	for j := 0; j < m.Size; j++ {
		m.Buf[j] = 0
	}
}

func NewVirtualMem(size int) *VirtualMem {
	vm := &VirtualMem{
		Size: size,
		Buf:  allocateBuffer(size),
	}
	runtime.SetFinalizer(vm, func(vm *VirtualMem) {
		err := internalMunmap(uintptr(unsafe.Pointer(&vm.Buf[0])), doubleSize(vm.Size))
		if err != nil {
			log.Panic(err)
		}
	})

	return vm
}

func AdjustBufferSize(size int) int {
	adjustedSize := math.Ceil((float64(size) / float64(pageSize))) * float64(pageSize)

	return int(adjustedSize)
}

func allocateBuffer(size int) []byte {
	nofd := ^uintptr(0)

	vaddr, err := internalMmap(0, doubleSize(size), syscall.MAP_SHARED|syscall.MAP_ANONYMOUS, nofd)
	if err != nil {
		log.Panic(err)
	}

	fileDescriptor, err := unix.MemfdCreate("magicbuffer", 0)
	if err != nil {
		log.Panic(err)
	}

	err = unix.Ftruncate(fileDescriptor, int64(size))
	if err != nil {
		log.Panic(err)
	}

	fd := uintptr(fileDescriptor)

	_, err = internalMmap(vaddr, size, syscall.MAP_SHARED|syscall.MAP_FIXED, fd)
	if err != nil {
		log.Panic(fmt.Errorf("first internal mmap failed: %w", err))
	}

	_, err = internalMmap(
		vaddr+uintptr(size), size, syscall.MAP_SHARED|syscall.MAP_FIXED, fd)
	if err != nil {
		log.Panic(fmt.Errorf("second internal mmap failed: %w", err))
	}

	sliceHeader := reflect.SliceHeader{
		Data: vaddr,
		Len:  doubleSize(size),
		Cap:  doubleSize(size),
	}

	buf := *(*[]byte)(unsafe.Pointer(&sliceHeader)) //nolint:govet // it is perfectly inteded use

	return buf
}

func internalMmap(addr uintptr, length, flags int, fd uintptr) (uintptr, error) {
	result, _, err := syscall.Syscall6(
		syscall.SYS_MMAP,
		addr,
		uintptr(length),
		uintptr(syscall.PROT_READ|syscall.PROT_WRITE),
		uintptr(flags),
		fd,
		uintptr(0),
	)
	if err != 0 {
		return 0, os.NewSyscallError("internalMMap error", err)
	}

	return result, nil
}

func internalMunmap(addr uintptr, length int) error {
	_, _, err := syscall.Syscall(
		syscall.SYS_MUNMAP,
		addr,
		uintptr(length),
		0,
	)
	if err != 0 {
		return os.NewSyscallError("internalMunmap error", err)
	}

	return nil
}
