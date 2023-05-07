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

package gain_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pawelgaczynski/gain"
	gainErrors "github.com/pawelgaczynski/gain/pkg/errors"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	"github.com/rs/zerolog"
	. "github.com/stretchr/testify/require"
)

type testServerConfig struct {
	protocol              string
	numberOfClients       int
	numberOfWorkers       int
	cpuAffinity           bool
	asyncHandler          bool
	goroutinePool         bool
	waitForDialAllClients bool
	afterDial             afterDialCallback
	writesCount           int

	readHandler onReadCallback
}

var defaultTestOnReadCallback = func(c gain.Conn, n int) {
	buffer := make([]byte, 128)

	_, err := c.Read(buffer)
	if err != nil {
		if errors.Is(err, gainErrors.ErrIsEmpty) {
			return
		}

		log.Panic(err)
	}

	if string(buffer[0:6]) != "cindex" {
		log.Panic(fmt.Errorf("unexpected data: %s", string(buffer[0:6])))
	}

	_, err = c.Write(append(buffer[0:10], []byte("TESTpayload12345")...))
	if err != nil {
		log.Panic(err)
	}
}

type callbacksHolder struct {
	onStartCallback  onStartCallback
	onAcceptCallback onAcceptCallback
	onReadCallback   onReadCallback
	onWriteCallback  onWriteCallback
	onCloseCallback  onCloseCallback
}

type testServerHandler struct {
	callbacksHolder

	onStartCount  atomic.Uint32
	onAcceptCount atomic.Uint32
	onReadCount   atomic.Uint32
	onWriteCount  atomic.Uint32
	onCloseCount  atomic.Uint32

	startedWg  *sync.WaitGroup
	onAcceptWg *sync.WaitGroup
	onReadWg   *sync.WaitGroup
	onWriteWg  *sync.WaitGroup
	onCloseWg  *sync.WaitGroup

	finished atomic.Bool
}

func (h *testServerHandler) OnStart(server gain.Server) {
	if !h.finished.Load() {
		h.startedWg.Done()

		if h.onStartCallback != nil {
			h.onStartCallback(server)
		}

		h.onStartCount.Add(1)
	}
}

func (h *testServerHandler) OnAccept(c gain.Conn) {
	if !h.finished.Load() {
		if h.onAcceptCallback != nil {
			h.onAcceptCallback(c)
		}

		h.onAcceptCount.Add(1)

		if h.onAcceptWg != nil {
			h.onAcceptWg.Done()
		}
	}
}

func (h *testServerHandler) OnClose(c gain.Conn, err error) {
	if !h.finished.Load() {
		if h.onCloseCallback != nil {
			h.onCloseCallback(c, err)
		}

		h.onCloseCount.Add(1)

		if h.onCloseWg != nil {
			h.onCloseWg.Done()
		}
	}
}

func (h *testServerHandler) OnRead(conn gain.Conn, n int) {
	if !h.finished.Load() {
		if h.onReadCallback != nil {
			h.onReadCallback(conn, n)
		}

		h.onReadCount.Add(1)

		if h.onReadWg != nil {
			h.onReadWg.Done()
		}
	}
}

func (h *testServerHandler) OnWrite(c gain.Conn, n int) {
	if !h.finished.Load() {
		if h.onWriteCallback != nil {
			h.onWriteCallback(c, n)
		}

		h.onWriteCount.Add(1)

		if h.onWriteWg != nil {
			h.onWriteWg.Done()
		}
	}
}

type afterDialCallback func(*testing.T, net.Conn, int, int)

var deafultAfterDial = func(t *testing.T, conn net.Conn, repeats, clientIndex int) {
	t.Helper()
	err := conn.SetDeadline(time.Now().Add(time.Second * 2))
	Nil(t, err)

	clientIndexBytes := []byte(fmt.Sprintf("cindex%04d", clientIndex))

	for i := 0; i < repeats; i++ {
		var bytesWritten int
		bytesWritten, err = conn.Write(append(clientIndexBytes, []byte("testdata1234567890")...))

		Nil(t, err)
		Equal(t, 28, bytesWritten)
		var buffer [64]byte
		var bytesRead int
		bytesRead, err = conn.Read(buffer[:])

		Nil(t, err)
		Equal(t, 26, bytesRead)
		Equal(t, string(append(clientIndexBytes, "TESTpayload12345"...)),
			string(buffer[:bytesRead]), "CONNFD: %d", getFdFromConn(conn))
	}
}

func dialClient(t *testing.T, protocol string, port int, clientConnChan chan net.Conn) {
	t.Helper()
	conn, err := net.DialTimeout(protocol, fmt.Sprintf("localhost:%d", port), time.Second)
	Nil(t, err)
	NotNil(t, conn)
	clientConnChan <- conn
}

func dialClientRW(t *testing.T, protocol string, port int,
	afterDial afterDialCallback, repeats, clientIndex int, clientConnChan chan net.Conn,
) {
	t.Helper()
	conn, err := net.DialTimeout(protocol, fmt.Sprintf("localhost:%d", port), 2*time.Second)
	Nil(t, err)
	NotNil(t, conn)
	afterDial(t, conn, repeats, clientIndex)
	clientConnChan <- conn
}

func newTestServerHandler(onReadCallback onReadCallback) *testServerHandler {
	testHandler := &testServerHandler{}

	var startedWg sync.WaitGroup

	startedWg.Add(1)
	testHandler.startedWg = &startedWg

	if onReadCallback != nil {
		testHandler.onReadCallback = onReadCallback
	} else {
		testHandler.onReadCallback = defaultTestOnReadCallback
	}

	return testHandler
}

func testServer(t *testing.T, testConfig testServerConfig, architecture gain.ServerArchitecture) {
	t.Helper()

	if testConfig.protocol == "" {
		log.Panic("network protocol is missing")
	}
	opts := []gain.ConfigOption{
		gain.WithLoggerLevel(zerolog.FatalLevel),
		gain.WithAsyncHandler(testConfig.asyncHandler),
		gain.WithGoroutinePool(testConfig.goroutinePool),
		gain.WithCPUAffinity(testConfig.cpuAffinity),
		gain.WithWorkers(testConfig.numberOfWorkers),
		gain.WithCBPF(false),
		gain.WithArchitecture(architecture),
	}

	config := gain.NewConfig(opts...)

	testHandler := newTestServerHandler(testConfig.readHandler)

	server := gain.NewServer(testHandler, config)

	defer func() {
		server.Shutdown()
	}()
	testPort := getTestPort()

	go func() {
		err := server.Start(fmt.Sprintf("%s://localhost:%d", testConfig.protocol, testPort))
		if err != nil {
			log.Panic(err)
		}
	}()

	clientConnChan := make(chan net.Conn, testConfig.numberOfClients)

	testHandler.startedWg.Wait()

	if testConfig.waitForDialAllClients {
		clientConnectWG := new(sync.WaitGroup)
		clientConnectWG.Add(testConfig.numberOfClients)
		testHandler.onAcceptCallback = func(c gain.Conn) {
			clientConnectWG.Done()
		}

		for i := 0; i < testConfig.numberOfClients; i++ {
			go dialClient(t, testConfig.protocol, testPort, clientConnChan)
		}
		clientConnectWG.Wait()
		Equal(t, testConfig.numberOfClients, server.ActiveConnections())

		for i := 0; i < testConfig.numberOfClients; i++ {
			conn := <-clientConnChan
			NotNil(t, conn)

			if tcpConn, ok := conn.(*net.TCPConn); ok {
				err := tcpConn.SetLinger(0)
				Nil(t, err)
			}
		}
	} else {
		var clientConnectWG *sync.WaitGroup
		if testConfig.protocol == gainNet.TCP {
			clientConnectWG = new(sync.WaitGroup)
			clientConnectWG.Add(testConfig.numberOfClients)
		}
		clientRWWG := new(sync.WaitGroup)
		if testConfig.writesCount == 0 {
			testConfig.writesCount = 1
		}
		clientRWWG.Add(testConfig.numberOfClients * testConfig.writesCount)
		if testConfig.protocol == gainNet.TCP {
			testHandler.onAcceptCallback = func(c gain.Conn) {
				clientConnectWG.Done()
			}
		}
		testHandler.onWriteCallback = func(c gain.Conn, n int) {
			clientRWWG.Done()
		}
		afterDial := deafultAfterDial
		if testConfig.afterDial != nil {
			afterDial = testConfig.afterDial
		}
		for i := 0; i < testConfig.numberOfClients; i++ {
			go func(clientIndex int) {
				dialClientRW(t, testConfig.protocol, testPort, afterDial, testConfig.writesCount, clientIndex, clientConnChan)
			}(i)
		}
		if testConfig.protocol == gainNet.TCP {
			clientConnectWG.Wait()
		}
		clientRWWG.Wait()
		for i := 0; i < testConfig.numberOfClients; i++ {
			conn := <-clientConnChan
			NotNil(t, conn)
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				err := tcpConn.SetLinger(0)
				Nil(t, err)
			}
		}
	}
}

var randomDataSize128 = make([]byte, 128)

type RingBufferTestDataHandler struct {
	t            *testing.T
	testFinished atomic.Bool
}

func (r *RingBufferTestDataHandler) OnRead(conn gain.Conn, _ int) {
	buffer := make([]byte, 128)
	bytesRead, readErr := conn.Read(buffer)

	if !r.testFinished.Load() {
		Equal(r.t, 128, bytesRead)

		if readErr != nil {
			log.Panic(readErr)
		}
		bytesWritten, writeErr := conn.Write(randomDataSize128)
		Equal(r.t, 128, bytesWritten)

		if writeErr != nil {
			log.Panic(writeErr)
		}
	}
}

func testRingBuffer(t *testing.T, protocol string, architecture gain.ServerArchitecture) {
	t.Helper()
	handler := RingBufferTestDataHandler{
		t: t,
	}
	bytesRandom, err := rand.Read(randomDataSize128)
	Nil(t, err)
	Equal(t, 128, bytesRandom)
	writesCount := 1000
	testServer(t, testServerConfig{
		numberOfClients: 1,
		numberOfWorkers: 1,
		protocol:        protocol,
		readHandler:     handler.OnRead,
		writesCount:     writesCount,
		afterDial: func(t *testing.T, conn net.Conn, _, _ int) {
			t.Helper()
			deadlineErr := conn.SetDeadline(time.Now().Add(time.Second * 1))
			Nil(t, deadlineErr)
			var buffer [256]byte
			for i := 0; i < writesCount; i++ {
				bytesWritten, writeErr := conn.Write(randomDataSize128)
				Nil(t, writeErr)
				Equal(t, 128, bytesWritten)
				bytesRead, readErr := conn.Read(buffer[:])
				Nil(t, readErr)
				Equal(t, 128, bytesRead)
				Equal(t, randomDataSize128, buffer[:bytesRead])
			}
			handler.testFinished.Store(true)
		},
	}, architecture)
}

func testCloseServer(t *testing.T, network string, architecture gain.ServerArchitecture, doubleShutdown bool) {
	t.Helper()
	testHandler := newConnServerTester(10, false)
	server, port := newTestConnServer(t, network, false, architecture, testHandler.testServerHandler)
	clientsGroup := newTestConnClientGroup(t, network, port, 10)
	clientsGroup.Dial()

	data := make([]byte, 512)

	_, err := rand.Read(data)
	Nil(t, err)
	clientsGroup.SetDeadline(time.Now().Add(time.Millisecond * 500))
	clientsGroup.Write(data)
	buffer := make([]byte, 512)

	clientsGroup.SetDeadline(time.Now().Add(time.Millisecond * 500))
	clientsGroup.Read(buffer)

	clientsGroup.SetDeadline(time.Time{})

	testHandler.waitForWrites()
	clientsGroup.Close()
	server.Shutdown()

	if doubleShutdown {
		server.Shutdown()
	}
}

func testCloseServerWithConnectedClients(t *testing.T, architecture gain.ServerArchitecture) {
	t.Helper()
	testHandler := newConnServerTester(10, false)
	server, port := newTestConnServer(t, gainNet.TCP, false, architecture, testHandler.testServerHandler)

	clientsGroup := newTestConnClientGroup(t, gainNet.TCP, port, 10)
	clientsGroup.Dial()

	data := make([]byte, 1024)
	_, err := rand.Read(data)
	Nil(t, err)
	clientsGroup.Write(data)
	buffer := make([]byte, 1024)
	clientsGroup.Read(buffer)

	testHandler.waitForWrites()
	server.Shutdown()
}

func testCloseConn(t *testing.T, async bool, architecture gain.ServerArchitecture, justClose bool) {
	t.Helper()
	testHandler := newTestServerHandler(func(conn gain.Conn, n int) {
		if !justClose {
			buf, err := conn.Next(n)
			if err != nil {
				log.Panic(err)
			}

			_, err = conn.Write(buf)
			if err != nil {
				log.Panic(err)
			}
		}

		err := conn.Close()
		if err != nil {
			log.Panic(err)
		}
	})

	server, port := newTestConnServer(t, gainNet.TCP, async, architecture, testHandler)

	var clientDoneWg sync.WaitGroup

	clientDoneWg.Add(1)

	go func(wg *sync.WaitGroup) {
		conn, cErr := net.DialTimeout(gainNet.TCP, fmt.Sprintf("localhost:%d", port), time.Second)
		Nil(t, cErr)
		NotNil(t, conn)
		testData := []byte("testdata1234567890")
		bytesN, cErr := conn.Write(testData)
		Nil(t, cErr)
		Equal(t, len(testData), bytesN)
		buffer := make([]byte, len(testData))
		bytesN, cErr = conn.Read(buffer)

		if !justClose {
			Nil(t, cErr)
			Equal(t, len(testData), bytesN)
			Equal(t, testData, buffer)
			bytesN, cErr = conn.Write(testData)
			Nil(t, cErr)
			Equal(t, len(testData), bytesN)
			bytesN, cErr = conn.Read(buffer)
		}

		Equal(t, io.EOF, cErr)
		Equal(t, 0, bytesN)
		wg.Done()
	}(&clientDoneWg)

	clientDoneWg.Wait()
	server.Shutdown()
}

func testLargeRead(t *testing.T, network string, architecture gain.ServerArchitecture) {
	t.Helper()

	doublePageSize := os.Getpagesize() * 4
	data := make([]byte, doublePageSize)
	_, err := rand.Read(data)
	Nil(t, err)

	var doneWg sync.WaitGroup

	doneWg.Add(1)
	onReadCallback := func(c gain.Conn, _ int) {
		readBuffer := make([]byte, doublePageSize)

		n, cErr := c.Read(readBuffer)
		if err != nil {
			log.Panic(cErr)
		}

		doneWg.Done()
		Equal(t, doublePageSize, n)

		n, cErr = c.Write(readBuffer)
		if cErr != nil {
			log.Panic(cErr)
		}

		Equal(t, doublePageSize, n)
	}

	testConnHandler := newTestServerHandler(onReadCallback)
	server, port := newTestConnServer(t, network, false, architecture, testConnHandler)

	clientsGroup := newTestConnClientGroup(t, network, port, 1)
	clientsGroup.Dial()

	clientsGroup.Write(data)
	buffer := make([]byte, len(data))
	clientsGroup.Read(buffer)

	Equal(t, data, buffer)

	doneWg.Wait()

	server.Shutdown()
}

func testMultipleReads(t *testing.T, network string, asyncHandler bool, architecture gain.ServerArchitecture) {
	t.Helper()

	pageSize := os.Getpagesize()
	data := make([]byte, pageSize)
	_, err := rand.Read(data)
	Nil(t, err)

	var (
		doneWg        sync.WaitGroup
		expectedReads int64 = 10
		readsCount    atomic.Int64
	)

	doneWg.Add(int(expectedReads))
	onReadCallback := func(c gain.Conn, _ int) {
		readBuffer := make([]byte, pageSize)

		n, cErr := c.Read(readBuffer)
		if err != nil {
			log.Panic(cErr)
		}

		readsCount.Add(1)
		doneWg.Done()
		Equal(t, pageSize, n)
	}

	testConnHandler := newTestServerHandler(onReadCallback)
	server, port := newTestConnServer(t, network, asyncHandler, architecture, testConnHandler)

	clientsGroup := newTestConnClientGroup(t, network, port, 1)
	clientsGroup.Dial()

	go func() {
		for i := 0; i < int(expectedReads); i++ {
			clientsGroup.Write(data)
			time.Sleep(time.Millisecond * 100)
		}
	}()

	doneWg.Wait()

	Equal(t, expectedReads, readsCount.Load())

	server.Shutdown()
}
