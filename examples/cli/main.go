// nolint:gochecknoinits,forbidigo
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/pawelgaczynski/gain"
	"github.com/pawelgaczynski/gain/logger"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

const (
	maxConnections = 1024
	bufferSize     = 4096
	port           = 9876
)

var (
	goMaxProcs             = runtime.NumCPU() * 2
	pprofReadHeaderTimeout = 3 * time.Second
)

var loggerLevelFlags = []string{
	"debug", "info", "warn", "error", "fatal", "panic", "disabled", "trace",
}

func loggerFlagToLevel(flag string) zerolog.Level {
	switch flag {
	case "debug":
		return logger.DebugLevel
	case "info":
		return logger.InfoLevel
	case "warn":
		return logger.WarnLevel
	case "error":
		return logger.ErrorLevel
	case "fatal":
		return logger.FatalLevel
	case "panic":
		return logger.PanicLevel
	case "disabled":
		return logger.Disabled
	case "trace":
		return logger.TraceLevel
	default:
		return logger.NoLevel
	}
}

type cmdConfig struct {
	sharding              bool
	workers               int
	maxConnections        int
	bufferSize            int
	port                  int
	prettyLogger          bool
	asyncHandler          bool
	goroutinePool         bool
	lockOSThread          bool
	cBPF                  bool
	batchSubmitter        bool
	processPriority       bool
	prefillConnectionPool bool
	loggerLevel           string
}

var config = &cmdConfig{}

var flags = []cli.Flag{
	&cli.BoolFlag{
		Name:        "sharding",
		Value:       false,
		Usage:       "sharding architecture",
		Destination: &config.sharding,
	},
	&cli.IntFlag{
		Name:        "workers",
		Value:       runtime.NumCPU(),
		Usage:       "number of workers",
		Destination: &config.workers,
	},
	&cli.IntFlag{
		Name:        "maxConnections",
		Value:       maxConnections,
		Usage:       "maximum number of connections",
		Destination: &config.maxConnections,
	},
	&cli.IntFlag{
		Name:        "bufferSize",
		Value:       bufferSize,
		Usage:       "size of read/write buffer",
		Destination: &config.bufferSize,
	},
	&cli.IntFlag{
		Name:        "port",
		Value:       port,
		Usage:       "listen TCP port",
		Destination: &config.port,
	},
	&cli.BoolFlag{
		Name:        "prettyLogger",
		Value:       false,
		Usage:       "print prettier logs. Warning: it can slow down server",
		Destination: &config.prettyLogger,
	},
	&cli.BoolFlag{
		Name:        "asyncHandler",
		Value:       false,
		Usage:       "use async handler for writes",
		Destination: &config.asyncHandler,
	},
	&cli.BoolFlag{
		Name:        "goroutinePool",
		Value:       false,
		Usage:       "use goroutine pool for async handler",
		Destination: &config.goroutinePool,
	},
	&cli.BoolFlag{
		Name:        "lockOSThread",
		Value:       false,
		Usage:       "lock OS thread for worker",
		Destination: &config.lockOSThread,
	},
	&cli.BoolFlag{
		Name:        "cbpf",
		Value:       false,
		Usage:       "use CBPF for bette packet locality",
		Destination: &config.cBPF,
	},
	&cli.BoolFlag{
		Name:        "batchSubmitter",
		Value:       false,
		Usage:       "use batch io_uring requests submitter instead of single submitter",
		Destination: &config.batchSubmitter,
	},
	&cli.BoolFlag{
		Name:        "processPriority",
		Value:       false,
		Usage:       "set high process priority. Note: requires root privileges",
		Destination: &config.processPriority,
	},
	&cli.BoolFlag{
		Name:        "prefillConnectionPool",
		Value:       false,
		Usage:       "prefill connection pool at start",
		Destination: &config.prefillConnectionPool,
	},
	&cli.StringFlag{
		Name:        "loggerLevel",
		Value:       "debug",
		Usage:       "logger level",
		Destination: &config.loggerLevel,
		Action: func(ctx *cli.Context, v string) error {
			contains := func(s []string, str string) bool {
				for _, v := range s {
					if v == str {
						return true
					}
				}
				return false
			}
			if !contains(loggerLevelFlags, v) {
				return fmt.Errorf("possible values for logger level: %v", loggerLevelFlags)
			}
			return nil
		},
	},
}

func getHTTPLastLine() []byte {
	return []byte("\r\nContent-Length: 13\r\n\r\nHello, World!")
}

var httpFirstLine = []byte("HTTP/1.1 200 OK\r\nServer: gnet\r\nContent-Type: text/plain\r\nDate: ")
var httpLastLine = getHTTPLastLine()

type Handler struct{}

var now atomic.Value

func ticktock() {
	now.Store(nowTimeFormat())
	for range time.Tick(time.Second) {
		now.Store(nowTimeFormat())
	}
}

func NowTimeFormat() string {
	return now.Load().(string)
}

func nowTimeFormat() string {
	return time.Now().Format("Mon, 02 Jan 2006 15:04:05 GMT")
}

func (h Handler) OnOpen(fd int) {}

func (h Handler) OnClose(fd int) {}

func (h Handler) OnData(c gain.Conn) error {
	_, err := c.Write(httpFirstLine)
	if err != nil {
		return err
	}
	_, err = c.Write([]byte(NowTimeFormat()))
	if err != nil {
		return err
	}
	_, err = c.Write(httpLastLine)
	if err != nil {
		return err
	}
	return nil
}

func (h Handler) AfterWrite(c gain.Conn) {}

func main() {
	runtime.GOMAXPROCS(goMaxProcs)
	go func() {
		server := &http.Server{
			Addr:              "0.0.0.0:6061",
			ReadHeaderTimeout: pprofReadHeaderTimeout,
		}
		log.Println(server.ListenAndServe())
	}()
	var server *gain.Server
	app := &cli.App{
		EnableBashCompletion: true,
		Flags:                flags,
		Name:                 "cli",
		Usage:                "Gain TCP server",
		Action: func(*cli.Context) error {
			opts := []gain.EngineOption{
				gain.WithMaxConn(uint(config.maxConnections)),
				gain.WithBufferSize(uint(config.bufferSize)),
				gain.WithPort(config.port),
				gain.WithLoggerLevel(loggerFlagToLevel(config.loggerLevel)),
				gain.WithPrettyLogger(config.prettyLogger),
				gain.WithAsyncHandler(config.asyncHandler),
				gain.WithGoroutinePool(config.goroutinePool),
				gain.WithLockOSThread(config.lockOSThread),
				gain.WithWorkers(config.workers),
				gain.WithCBPF(config.cBPF),
				gain.WithBatchSubmitter(config.batchSubmitter),
				gain.WithSharding(config.sharding),
				gain.WithProcessPriority(config.processPriority),
				gain.WithPrefillConnectionPool(config.prefillConnectionPool),
			}
			config := gain.NewConfig(opts...)
			fmt.Printf("Configuration: %+v \n", config)
			fmt.Println("Starting Gain TCP Server...")
			server = gain.NewServer(config, Handler{})
			return server.Start()
		},
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("Server closing...")
		server.Close()
	}()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func init() {
	now.Store(nowTimeFormat())
	go ticktock()
}
