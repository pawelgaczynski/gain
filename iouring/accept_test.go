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

package iouring_test

import (
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/pawelgaczynski/gain/iouring"
	. "github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestAccept(t *testing.T) {
	ring, err := iouring.CreateRing()
	Nil(t, err)
	defer ring.Close()

	socketFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	Nil(t, err)
	err = syscall.SetsockoptInt(socketFd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	Nil(t, err)
	err = syscall.Bind(socketFd, &syscall.SockaddrInet4{
		Port: testPort,
	})
	Nil(t, err)
	err = syscall.SetNonblock(socketFd, false)
	Nil(t, err)
	err = syscall.Listen(socketFd, 1)
	Nil(t, err)
	defer func() {
		err := syscall.Close(socketFd)
		Nil(t, err)
	}()

	entry, err := ring.GetSQE()
	Nil(t, err)
	var clientLen = new(uint32)
	clientAddr := &unix.RawSockaddrAny{}
	*clientLen = unix.SizeofSockaddrAny
	clientAddrPointer := uintptr(unsafe.Pointer(clientAddr))
	clientLenPointer := uint64(uintptr(unsafe.Pointer(clientLen)))

	entry.PrepareAccept(int(uintptr(socketFd)), clientAddrPointer, clientLenPointer, 0)
	entry.UserData = uint64(socketFd)
	cqeNr, err := ring.Submit()
	Nil(t, err)
	Equal(t, uint(1), cqeNr)

	clientConnChan := make(chan net.Conn)
	go func() {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", testPort), time.Second)
		Nil(t, err)
		clientConnChan <- conn
	}()
	defer func() {
		conn := <-clientConnChan
		err = conn.(*net.TCPConn).SetLinger(0)
		Nil(t, err)
	}()

	cqes := make([]*iouring.CompletionQueueEvent, 128)
	Nil(t, err)
	for {
		n := ring.PeekBatchCQE(cqes)
		for i := 0; i < n; i++ {
			cqe := cqes[i]
			Equal(t, uint64(socketFd), cqe.UserData())
			Greater(t, cqe.Res(), int32(0))
			err = syscall.Shutdown(int(cqe.Res()), syscall.SHUT_RDWR)
			Nil(t, err)
		}
		if n > 0 {
			ring.CQAdvance(uint32(n))
			Nil(t, err)
			break
		}
	}
}
