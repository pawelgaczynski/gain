package iouring_test

import (
	"errors"
	"fmt"
	"log"
	"net"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/pawelgaczynski/gain/iouring"
	. "github.com/stretchr/testify/require"
)

const (
	udpRecv = iota
	udpSend
)

func anyToSockaddrInet4(rsa *syscall.RawSockaddrAny) (*syscall.SockaddrInet4, error) {
	if rsa == nil {
		return nil, syscall.EINVAL
	}

	if rsa.Addr.Family != syscall.AF_INET {
		return nil, syscall.EAFNOSUPPORT
	}

	rsaPointer := (*syscall.RawSockaddrInet4)(unsafe.Pointer(rsa))
	sockAddr := new(syscall.SockaddrInet4)
	p := (*[2]byte)(unsafe.Pointer(&rsaPointer.Port))
	sockAddr.Port = int(p[0])<<8 + int(p[1])

	for i := 0; i < len(sockAddr.Addr); i++ {
		sockAddr.Addr[i] = rsaPointer.Addr[i]
	}

	return sockAddr, nil
}

type udpConnection struct {
	fd            uint64
	buffer        []byte
	controlBuffer []byte
	state         int
	msg           *syscall.Msghdr
	rsa           *syscall.RawSockaddrAny
}

func udpLoop(t *testing.T, ring *iouring.Ring, socketFd int, connection *udpConnection) bool {
	t.Helper()

	cqe, err := ring.WaitCQE()
	if errors.Is(err, iouring.ErrAgain) || errors.Is(err, iouring.ErrInterrupredSyscall) ||
		errors.Is(err, iouring.ErrTimerExpired) {
		return false
	}

	Nil(t, err)
	entry, err := ring.GetSQE()
	Nil(t, err)
	ring.CQESeen(cqe)

	switch connection.state {
	case udpRecv:
		_, err = anyToSockaddrInet4(connection.rsa)
		if err != nil {
			log.Panic(err)
		}

		Equal(t, "testdata1234567890", string(connection.buffer[:18]))
		connection.buffer = connection.buffer[:0]
		data := []byte("responsedata0123456789")
		copied := copy(connection.buffer[:len(data)], data)
		Equal(t, 22, copied)
		buffer := connection.buffer[:len(data)]

		connection.msg.Iov.Base = (*byte)(unsafe.Pointer(&buffer[0]))
		connection.msg.Iov.SetLen(len(buffer))
		entry.PrepareSendMsg(socketFd, connection.msg, 0)

		entry.UserData = connection.fd
		connection.state = udpSend

	case udpSend:
		Equal(t, connection.fd, cqe.UserData())
		Equal(t, cqe.Res(), int32(22))

		return true
	}
	cqeNr, err := ring.Submit()
	Nil(t, err)
	Equal(t, uint(1), cqeNr)

	return false
}

func TestUDPRecvSend(t *testing.T) {
	ring, err := iouring.CreateRing(16)
	Nil(t, err)

	defer func() {
		_ = ring.QueueExit()
	}()

	socketFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	Nil(t, err)
	err = syscall.SetsockoptInt(socketFd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	Nil(t, err)
	testPort := getTestPort()
	err = syscall.Bind(socketFd, &syscall.SockaddrInet4{
		Port: testPort,
	})
	Nil(t, err)
	err = syscall.SetNonblock(socketFd, false)
	Nil(t, err)

	defer func() {
		closerErr := syscall.Close(socketFd)
		Nil(t, closerErr)
	}()

	entry, err := ring.GetSQE()
	Nil(t, err)

	buffer := make([]byte, 64)

	var iovec syscall.Iovec
	iovec.Base = (*byte)(unsafe.Pointer(&buffer[0]))
	iovec.SetLen(len(buffer))

	controlBuffer := make([]byte, 64)

	var (
		msg syscall.Msghdr
		rsa syscall.RawSockaddrAny
	)
	msg.Name = (*byte)(unsafe.Pointer(&rsa))
	msg.Namelen = uint32(syscall.SizeofSockaddrAny)
	msg.Iov = &iovec
	msg.Iovlen = 1
	msg.Control = (*byte)(unsafe.Pointer(&controlBuffer[0]))
	msg.SetControllen(len(controlBuffer))

	entry.PrepareRecvMsg(int(uintptr(socketFd)), &msg, 0)
	entry.UserData = uint64(socketFd)

	cqeNr, err := ring.Submit()
	Nil(t, err)
	Equal(t, uint(1), cqeNr)

	connection := &udpConnection{state: udpRecv, buffer: buffer, msg: &msg, rsa: &rsa, controlBuffer: controlBuffer}

	clientConnChan := make(chan net.Conn)
	go func() {
		conn, cErr := net.DialTimeout("udp", fmt.Sprintf("127.0.0.1:%d", testPort), time.Second)
		Nil(t, cErr)
		NotNil(t, conn)
		bytesWritten, cErr := conn.Write([]byte("testdata1234567890"))
		Nil(t, cErr)
		Equal(t, 18, bytesWritten)

		var buffer [22]byte
		bytesWritten, cErr = conn.Read(buffer[:])
		Nil(t, cErr)
		Equal(t, 22, bytesWritten)
		Equal(t, "responsedata0123456789", string(buffer[:]))
		clientConnChan <- conn
	}()

	defer func() {
		<-clientConnChan
	}()

	for {
		if udpLoop(t, ring, socketFd, connection) {
			break
		}
	}
}
