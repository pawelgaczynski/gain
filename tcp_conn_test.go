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
	"sync"
	"testing"
	"time"

	"github.com/pawelgaczynski/gain"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	. "github.com/stretchr/testify/require"
)

func testConnectionHandling(t *testing.T, architecture gain.ServerArchitecture) {
	t.Helper()
	testHandler := newConnServerTester(gainNet.TCP, 0, false)
	server, port := newTestConnServer(t, gainNet.TCP, false, architecture, testHandler.testServerHandler)

	defer func() {
		server.Shutdown()
	}()

	var connWaitGroup sync.WaitGroup

	connWaitGroup.Add(10)
	testHandler.onAcceptCallback = func(_ gain.Conn, _ string) {
		connWaitGroup.Done()
	}

	clientsGroup := newTestConnClientGroup(t, gainNet.TCP, port, 10)
	clientsGroup.Dial()

	connWaitGroup.Wait()

	Equal(t, 10, server.ActiveConnections())

	clientsGroup.Close()

	time.Sleep(1 * time.Second)

	Equal(t, 0, server.ActiveConnections())
}
