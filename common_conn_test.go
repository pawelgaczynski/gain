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
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/pawelgaczynski/gain"
	"github.com/rs/zerolog"
	. "github.com/stretchr/testify/require"
)

type connServerTester struct {
	*testServerHandler
	mutex                  sync.Mutex
	writeWG                *sync.WaitGroup
	writeCount             uint32
	targetWriteCount       uint32
	removeWGAfterMinWrites bool
}

func (t *connServerTester) waitForWrites() {
	t.writeWG.Wait()
}

func (t *connServerTester) onReadCallback(conn gain.Conn) {
	buf, _ := conn.Next(-1)
	_, _ = conn.Write(buf)
}

func (t *connServerTester) onWriteCallback(_ gain.Conn) {
	if t.writeWG != nil {
		t.mutex.Lock()

		t.writeCount++
		if t.writeCount >= t.targetWriteCount {
			t.writeWG.Done()

			if t.removeWGAfterMinWrites {
				t.writeWG = nil
			}
		}
		t.mutex.Unlock()
	}
}

func newConnServerTester(writeCount int, removeWGAfterMinWrites bool) *connServerTester {
	connServerTester := &connServerTester{}

	if writeCount > 0 {
		var writeWG sync.WaitGroup

		writeWG.Add(1)
		connServerTester.writeWG = &writeWG
		connServerTester.targetWriteCount = uint32(writeCount)
		connServerTester.removeWGAfterMinWrites = removeWGAfterMinWrites
	}

	testConnHandler := newTestServerHandler(connServerTester.onReadCallback)

	testConnHandler.onWriteCallback = connServerTester.onWriteCallback
	connServerTester.testServerHandler = testConnHandler

	return connServerTester
}

type testConnClient struct {
	t       *testing.T
	conn    net.Conn
	network string
	port    int
}

func (c *testConnClient) Dial() {
	conn, err := net.DialTimeout(c.network, fmt.Sprintf("localhost:%d", c.port), time.Second)
	Nil(c.t, err)
	NotNil(c.t, conn)
	c.conn = conn
}

func (c *testConnClient) Close() {
	err := c.conn.Close()
	Nil(c.t, err)
}

func (c *testConnClient) SetDeadline(t time.Time) {
	err := c.conn.SetDeadline(t)
	Nil(c.t, err)
}

func (c *testConnClient) Write(buffer []byte) {
	bytesWritten, writeErr := c.conn.Write(buffer)
	Nil(c.t, writeErr)
	Equal(c.t, len(buffer), bytesWritten)
}

func (c *testConnClient) Read(buffer []byte) {
	bytesRead, readErr := c.conn.Read(buffer)
	Nil(c.t, readErr)
	Equal(c.t, len(buffer), bytesRead)
}

func newTestConnClient(t *testing.T, network string, port int) *testConnClient {
	t.Helper()

	return &testConnClient{
		t:       t,
		network: network,
		port:    port,
	}
}

type testConnClientGroup struct {
	clients []*testConnClient
}

func (c *testConnClientGroup) Dial() {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].Dial()
	}
}

func (c *testConnClientGroup) Close() {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].Close()
	}
}

func (c *testConnClientGroup) SetDeadline(t time.Time) {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].SetDeadline(t)
	}
}

func (c *testConnClientGroup) Write(buffer []byte) {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].Write(buffer)
	}
}

func (c *testConnClientGroup) Read(buffer []byte) {
	for i := 0; i < len(c.clients); i++ {
		c.clients[i].Read(buffer)
	}
}

func newTestConnClientGroup(t *testing.T, network string, port int, n int) *testConnClientGroup {
	t.Helper()
	group := &testConnClientGroup{
		clients: make([]*testConnClient, n),
	}

	for i := 0; i < n; i++ {
		group.clients[i] = newTestConnClient(t, network, port)
	}

	return group
}

func newTestConnServer(
	t *testing.T, network string, async bool, architecture gain.ServerArchitecture, eventHandler *testServerHandler,
) (*gain.Server, int) {
	t.Helper()
	opts := []gain.ConfigOption{
		gain.WithLoggerLevel(zerolog.FatalLevel),
		gain.WithWorkers(4),
		gain.WithArchitecture(architecture),
		gain.WithAsyncHandler(async),
	}

	opts = append(opts, []gain.ConfigOption{}...)
	config := gain.NewConfig(opts...)

	server := gain.NewServer(eventHandler, config)
	testPort := getTestPort()

	go func() {
		err := server.Start(fmt.Sprintf("%s://localhost:%d", network, testPort))
		if err != nil {
			log.Panic(err)
		}
	}()

	eventHandler.startedWg.Wait()

	return server, int(port)
}
