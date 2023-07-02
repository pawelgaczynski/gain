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
	"runtime"
	"time"

	"github.com/rs/zerolog"
)

const (
	defaultPort           = 8080
	defaultMaxCQEvents    = 16384
	defaultMaxSQEntries   = 16384
	defaultRecvBufferSize = 4096
	defaultSendBufferSize = 4096
)

type Option[T any] func(*T)

type ConfigOption Option[Config]

type ServerArchitecture int

const (
	// Reactor design pattern has one input called Acceptor,
	// which demultiplexes the handling of incoming connections to Consumer workers.
	// The load balancing algorithm can be selected via configuration option.
	Reactor ServerArchitecture = iota
	// The Socket Sharding  allow multiple workers to listen on the same address and port combination.
	// In this case the kernel distributes incoming requests across all the sockets.
	SocketSharding
)

// Config is the configuration for the gain engine.
type Config struct {
	// Architecture indicates one of the two available architectures: Reactor and SocketSharding.
	//
	// The Reactor design pattern has one input called Acceptor,
	// which demultiplexes the handling of incoming connections to Consumer workers.
	// The load balancing algorithm can be selected via configuration option.
	//
	// The Socket Sharding allows multiple workers to listen on the same address and port combination.
	// In this case the kernel distributes incoming requests across all the sockets.
	Architecture ServerArchitecture
	// AsyncHandler indicates whether the engine should run the OnRead EventHandler method in a separate goroutines.
	AsyncHandler bool
	// GoroutinePool indicates use of pool of bounded goroutines for OnRead calls.
	// Important: Valid only if AsyncHandler is true
	GoroutinePool bool
	// CPUAffinity determines whether each engine worker is locked to the one CPU.
	CPUAffinity bool
	// ProcessPriority sets the prority of the process to high (-19). Requires root privileges.
	ProcessPriority bool
	// Workers indicates the number of consumers or shard workers. The default is runtime.NumCPU().
	Workers int
	// CBPFilter uses custom BPF filter to improve the performance of the Socket Sharding architecture.
	CBPFilter bool
	// LoadBalancing indicates the load-balancing algorithm to use when assigning a new connection.
	// Important: valid only for Reactor architecture.
	LoadBalancing LoadBalancing
	// SocketRecvBufferSize sets the maximum socket receive buffer in bytes.
	SocketRecvBufferSize int
	// SocketSendBufferSize sets the maximum socket send buffer in bytes.
	SocketSendBufferSize int
	// TCPKeepAlive sets the TCP keep-alive for the socket.
	TCPKeepAlive time.Duration
	// LoggerLevel indicates the logging level.
	LoggerLevel zerolog.Level
	// PrettyLogger sets the pretty-printing zerolog mode.
	// Important: it is inefficient so should be used only for debugging.
	PrettyLogger bool

	// ==============================
	// io_uring related options
	// ==============================
	// MaxSQEntries sets the maximum number of SQEs that can be submitted in one batch.
	// If the number of SQEs exceeds this value, the io_uring will return a SQE overflow error.
	MaxSQEntries uint
	// MaxCQEvents sets the maximum number of CQEs that can be retrieved in one batch.
	MaxCQEvents uint
}

// WithArchitecture sets the architecture of gain engine.
func WithArchitecture(architecture ServerArchitecture) ConfigOption {
	return func(c *Config) {
		c.Architecture = architecture
	}
}

// WithAsyncHandler sets the asynchronous mode for the OnRead callback.
func WithAsyncHandler(asyncHandler bool) ConfigOption {
	return func(c *Config) {
		c.AsyncHandler = asyncHandler
	}
}

// WithGoroutinePool sets the goroutine pool for asynchronous handler.
func WithGoroutinePool(goroutinePool bool) ConfigOption {
	return func(c *Config) {
		c.GoroutinePool = goroutinePool
	}
}

// WithCPUAffinity sets the CPU affinity option.
func WithCPUAffinity(cpuAffinity bool) ConfigOption {
	return func(c *Config) {
		c.CPUAffinity = cpuAffinity
	}
}

// WithProcessPriority sets the high process priority. Note: requires root privileges.
func WithProcessPriority(processPriority bool) ConfigOption {
	return func(c *Config) {
		c.ProcessPriority = processPriority
	}
}

// WithWorkers sets the number of workers.
func WithWorkers(workers int) ConfigOption {
	return func(c *Config) {
		c.Workers = workers
	}
}

// WithCBPF sets the CBPF filter for the gain engine.
func WithCBPF(cbpf bool) ConfigOption {
	return func(c *Config) {
		c.CBPFilter = cbpf
	}
}

// WithLoadBalancing sets the load balancing algorithm.
func WithLoadBalancing(loadBalancing LoadBalancing) ConfigOption {
	return func(c *Config) {
		c.LoadBalancing = loadBalancing
	}
}

// WithSocketRecvBufferSize sets the maximum socket receive buffer in bytes.
func WithSocketRecvBufferSize(size int) ConfigOption {
	return func(c *Config) {
		c.SocketRecvBufferSize = size
	}
}

// WithSocketSendBufferSize sets the maximum socket send buffer in bytes.
func WithSocketSendBufferSize(size int) ConfigOption {
	return func(c *Config) {
		c.SocketSendBufferSize = size
	}
}

// WithTCPKeepAlive sets the TCP keep-alive for the socket.
func WithTCPKeepAlive(tcpKeepAlive time.Duration) ConfigOption {
	return func(c *Config) {
		c.TCPKeepAlive = tcpKeepAlive
	}
}

// WithLoggerLevel sets the logging level.
func WithLoggerLevel(loggerLevel zerolog.Level) ConfigOption {
	return func(c *Config) {
		c.LoggerLevel = loggerLevel
	}
}

// WithPrettyLogger sets the pretty-printing zerolog mode.
func WithPrettyLogger(prettyLogger bool) ConfigOption {
	return func(c *Config) {
		c.PrettyLogger = prettyLogger
	}
}

// WithMaxSQEntries sets the maximum number of entries in the submission queue.
func WithMaxSQEntries(maxSQEntries uint) ConfigOption {
	return func(c *Config) {
		c.MaxSQEntries = maxSQEntries
	}
}

// WithMaxCQEvents sets the maximum number of entries in the completion queue.
func WithMaxCQEvents(maxCQEvents uint) ConfigOption {
	return func(c *Config) {
		c.MaxCQEvents = maxCQEvents
	}
}

func NewConfig(opts ...ConfigOption) Config {
	config := Config{
		Architecture:         Reactor,
		AsyncHandler:         false,
		GoroutinePool:        false,
		CPUAffinity:          false,
		ProcessPriority:      false,
		LoggerLevel:          zerolog.ErrorLevel,
		PrettyLogger:         false,
		Workers:              runtime.NumCPU(),
		CBPFilter:            false,
		LoadBalancing:        RoundRobin,
		SocketRecvBufferSize: 0,
		SocketSendBufferSize: 0,
		TCPKeepAlive:         0,
		MaxSQEntries:         defaultMaxSQEntries,
		MaxCQEvents:          defaultMaxCQEvents,
	}
	for _, opt := range opts {
		opt(&config)
	}

	return config
}
