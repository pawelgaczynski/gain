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

package gain

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	. "github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	opts := []ConfigOption{
		WithArchitecture(SocketSharding),
		WithAsyncHandler(true),
		WithGoroutinePool(true),
		WithCPUAffinity(true),
		WithProcessPriority(true),
		WithWorkers(128),
		WithCBPF(true),
		WithLoadBalancing(LeastConnections),
		WithSocketRecvBufferSize(4096),
		WithSocketSendBufferSize(4096),
		WithTCPKeepAlive(time.Second),
		WithLoggerLevel(zerolog.DebugLevel),
		WithPrettyLogger(true),
	}

	config := NewConfig(opts...)

	Equal(t, SocketSharding, config.Architecture)
	Equal(t, true, config.AsyncHandler)
	Equal(t, true, config.GoroutinePool)
	Equal(t, true, config.CPUAffinity)
	Equal(t, true, config.ProcessPriority)
	Equal(t, 128, config.Workers)
	Equal(t, true, config.CBPFilter)
	Equal(t, LeastConnections, config.LoadBalancing)
	Equal(t, 4096, config.SocketRecvBufferSize)
	Equal(t, 4096, config.SocketSendBufferSize)
	Equal(t, time.Second, config.TCPKeepAlive)
	Equal(t, zerolog.DebugLevel, config.LoggerLevel)
	Equal(t, true, config.PrettyLogger)
}
