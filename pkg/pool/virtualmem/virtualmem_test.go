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
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"reflect"
	"syscall"
	"testing"
	"unsafe"

	. "github.com/stretchr/testify/require"
)

func deallocate(data []byte, size int) error {
	return internalMunmap(uintptr(unsafe.Pointer(&data[0])), size*2)
}

func allocateTempfileBuffer(size int) []byte {
	nofd := ^uintptr(0)

	vaddr, err := internalMmap(
		0, 2*size, syscall.MAP_SHARED|syscall.MAP_ANONYMOUS, nofd)
	if err != nil {
		log.Panic(err)
	}

	file, err := ioutil.TempFile("", "magicbuffer")
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	if err = os.Remove(file.Name()); err != nil {
		log.Panic(err)
	}

	fd := file.Fd()
	if err = syscall.Ftruncate(int(fd), int64(size)); err != nil {
		log.Panic(err)
	}

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
		Len:  2 * size,
		Cap:  2 * size,
	}
	buf := *(*[]byte)(unsafe.Pointer(&sliceHeader)) //nolint:govet

	return buf
}

func BenchmarkVMTempFile(b *testing.B) {
	var (
		pagesize = os.Getpagesize()
		err      error
	)

	for i := 0; i < b.N; i++ {
		data := allocateTempfileBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}
}

func BenchmarkVMMemfd(b *testing.B) {
	var (
		pagesize = os.Getpagesize()
		err      error
	)

	for i := 0; i < b.N; i++ {
		data := allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}
}

var printMapsAndFds = os.Getenv("TEST_PRINT_MAPS_AND_FDS") == "true"

var printCmd = func(label, cmd string) {
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Panic(err)
	}

	if printMapsAndFds {
		fmt.Printf("%s: %s\n", label, string(out)) //nolint:forbidigo
	}
}

func TestMemfdMMapAndUnmap(_ *testing.T) {
	pagesize := os.Getpagesize()
	pid := os.Getpid()
	listCmd := fmt.Sprintf("cat /proc/%d/maps", pid)
	countCmd := fmt.Sprintf("cat /proc/%d/maps | wc -l", pid)
	fdCmd := fmt.Sprintf("ls -l /proc/%d/fd | wc -l", pid)

	printCmd("ulimit -a", "ulimit -a")
	printCmd("max map count", "sysctl vm.max_map_count")

	printList := func() {
		printCmd("list of proc maps", listCmd)
	}
	printCount := func() {
		printCmd("count of proc maps", countCmd)
	}
	printFds := func() {
		printCmd("count of proc fds", fdCmd)
	}

	var (
		data []byte
		err  error
	)

	printCount()
	printFds()

	for i := 0; i < 1; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 100; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 1000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 10000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 30000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()

	for i := 0; i < 60000; i++ {
		data = allocateBuffer(pagesize)
		if err = deallocate(data, pagesize); err != nil {
			log.Panic(err)
		}
	}

	printCount()
	printFds()
	printList()
}

func TestVirtualMem(t *testing.T) {
	vm := NewVirtualMem(os.Getpagesize())
	NotNil(t, vm)
	Equal(t, os.Getpagesize(), vm.Size)

	Equal(t, os.Getpagesize()*2, len(vm.Buf))

	dataSize := os.Getpagesize()
	data := make([]byte, dataSize)
	_, err := rand.Read(data)
	Nil(t, err)

	for i := range vm.Buf {
		Zero(t, vm.Buf[i])
	}

	for i := 0; i < vm.Size; i++ {
		vm.Buf[i] = data[i]
	}

	for i := 0; i < vm.Size; i++ {
		Equal(t, vm.Buf[i], vm.Buf[i+vm.Size])
	}

	vm.Zeroes()

	for i := range vm.Buf {
		Zero(t, vm.Buf[i])
	}

	for i := 0; i < vm.Size; i++ {
		vm.Buf[i+vm.Size] = data[i]
	}

	for i := 0; i < vm.Size; i++ {
		Equal(t, vm.Buf[i], vm.Buf[i+vm.Size])
	}

	vm.Zeroes()

	for i := range vm.Buf {
		Zero(t, vm.Buf[i])
	}
}
