package gain

import (
	"runtime"

	"github.com/rs/zerolog"
)

const (
	defaultPort          = 8080
	defaultMaxCQEvents   = 512
	defaultMaxConn       = 4096
	defaultMaxBufferSize = 4096
)

type ConfigOption[T any] func(*T)

type EngineOption ConfigOption[Config]

type ServerArchitecture int

const (
	Reactor ServerArchitecture = iota
	Sharding
)

type Config struct {
	sharding              bool
	maxConn               uint
	bufferSize            uint
	asyncHandler          bool
	goroutinePool         bool
	lockOSThread          bool
	processPriority       bool
	batchSubmitter        bool
	prefillConnectionPool bool
	loggerLevel           zerolog.Level
	prettyLogger          bool
	workers               int
	cbpf                  bool
	port                  int
	loadBalancing         LoadBalancing
}

func WithSharding(sharding bool) EngineOption {
	return func(c *Config) {
		c.sharding = sharding
	}
}

func WithMaxConn(maxConn uint) EngineOption {
	return func(c *Config) {
		c.maxConn = maxConn
	}
}

func WithLoadBalancing(loadBalancing LoadBalancing) EngineOption {
	return func(c *Config) {
		c.loadBalancing = loadBalancing
	}
}

func WithBufferSize(bufferSize uint) EngineOption {
	return func(c *Config) {
		c.bufferSize = bufferSize
	}
}

func WithAsyncHandler(asyncHandler bool) EngineOption {
	return func(c *Config) {
		c.asyncHandler = asyncHandler
	}
}

func WithGoroutinePool(goroutinePool bool) EngineOption {
	return func(c *Config) {
		c.goroutinePool = goroutinePool
	}
}

func WithBatchSubmitter(batchSubmitter bool) EngineOption {
	return func(c *Config) {
		c.batchSubmitter = batchSubmitter
	}
}

func WithPrefillConnectionPool(prefillConnectionPool bool) EngineOption {
	return func(c *Config) {
		c.prefillConnectionPool = prefillConnectionPool
	}
}

func WithWorkers(workers int) EngineOption {
	return func(c *Config) {
		c.workers = workers
	}
}

func WithCBPF(cbpf bool) EngineOption {
	return func(c *Config) {
		c.cbpf = cbpf
	}
}

func WithLockOSThread(lockOSThread bool) EngineOption {
	return func(c *Config) {
		c.lockOSThread = lockOSThread
	}
}

func WithProcessPriority(processPriority bool) EngineOption {
	return func(c *Config) {
		c.processPriority = processPriority
	}
}

func WithPort(port int) EngineOption {
	return func(c *Config) {
		c.port = port
	}
}

func WithLoggerLevel(loggerLevel zerolog.Level) EngineOption {
	return func(c *Config) {
		c.loggerLevel = loggerLevel
	}
}

func WithPrettyLogger(prettyLogger bool) EngineOption {
	return func(c *Config) {
		c.prettyLogger = prettyLogger
	}
}

func NewConfig(opts ...EngineOption) Config {
	config := Config{
		sharding:              false,
		workers:               runtime.NumCPU(),
		port:                  defaultPort,
		maxConn:               defaultMaxConn,
		bufferSize:            defaultMaxBufferSize,
		lockOSThread:          false,
		loggerLevel:           zerolog.ErrorLevel,
		prettyLogger:          false,
		asyncHandler:          false,
		goroutinePool:         false,
		batchSubmitter:        false,
		prefillConnectionPool: false,
		loadBalancing:         RoundRobin,
	}
	for _, opt := range opts {
		opt(&config)
	}
	return config
}
