//nolint:gochecknoinits,forbidigo,goerr113,forcetypeassert,wrapcheck
package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" //nolint:gosec
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/pawelgaczynski/gain"
	gainNet "github.com/pawelgaczynski/gain/pkg/net"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

const (
	port = 9000
)

var (
	goMaxProcs             = runtime.NumCPU() * 2
	pprofReadHeaderTimeout = 3 * time.Second
)

var architectures = []string{
	"reactor", "sharding",
}

var allowedNetworks = []string{
	gainNet.TCP, gainNet.UDP,
}

var handlers = []string{
	"http", "echo",
}

type cmdConfig struct {
	gain.Config
	port    int
	handler string
	network string
}

var contains = func(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

var config = &cmdConfig{}

var flags = []cli.Flag{
	&cli.StringFlag{
		Name:  "architecture",
		Value: "reactor",
		Usage: "server architecture",
		Action: func(ctx *cli.Context, v string) error {
			if !contains(architectures, v) {
				return fmt.Errorf("possible values for architectures: %v", architectures)
			}
			switch v {
			case "reactor":
				config.Architecture = gain.Reactor
			case "sharding":
				config.Architecture = gain.SocketSharding
			}

			return nil
		},
	},
	&cli.IntFlag{
		Name:        "workers",
		Value:       runtime.NumCPU(),
		Usage:       "number of workers",
		Destination: &config.Workers,
	},
	&cli.IntFlag{
		Name:        "port",
		Value:       port,
		Usage:       "listen port",
		Destination: &config.port,
	},
	&cli.BoolFlag{
		Name:        "prettyLogger",
		Value:       false,
		Usage:       "print prettier logs. Warning: it can slow down server",
		Destination: &config.PrettyLogger,
	},
	&cli.BoolFlag{
		Name:        "asyncHandler",
		Value:       false,
		Usage:       "use async handler for writes",
		Destination: &config.AsyncHandler,
	},
	&cli.BoolFlag{
		Name:        "goroutinePool",
		Value:       false,
		Usage:       "use goroutine pool for async handler",
		Destination: &config.GoroutinePool,
	},
	&cli.BoolFlag{
		Name:        "cpuAffinity",
		Value:       false,
		Usage:       "set CPU Affinity",
		Destination: &config.CPUAffinity,
	},
	&cli.BoolFlag{
		Name:        "cbpf",
		Value:       false,
		Usage:       "use CBPF for bette packet locality",
		Destination: &config.CBPFilter,
	},
	&cli.BoolFlag{
		Name:        "processPriority",
		Value:       false,
		Usage:       "set high process priority. Note: requires root privileges",
		Destination: &config.ProcessPriority,
	},
	&cli.StringFlag{
		Name:  "loggerLevel",
		Value: "error",
		Usage: "logger level",
		Action: func(ctx *cli.Context, v string) error {
			var err error
			config.LoggerLevel, err = zerolog.ParseLevel(v)

			return err
		},
	},
	&cli.StringFlag{
		Name:        "handler",
		Value:       "echo",
		Usage:       "handler type. Allowed values are 'http' and 'echo'",
		Destination: &config.handler,
		Action: func(ctx *cli.Context, v string) error {
			if !contains(handlers, v) {
				return fmt.Errorf("possible values for handler: %v", handlers)
			}

			return nil
		},
	},
	&cli.StringFlag{
		Name:        "network",
		Value:       gainNet.TCP,
		Usage:       "network type",
		Destination: &config.network,
		Action: func(ctx *cli.Context, v string) error {
			if !contains(allowedNetworks, v) {
				return fmt.Errorf("possible values for networks: %v", allowedNetworks)
			}

			return nil
		},
	},
}

func getHTTPLastLine() []byte {
	return []byte("\r\nContent-Length: 13\r\n\r\nHello, World!")
}

var (
	httpFirstLine = []byte("HTTP/1.1 200 OK\r\nServer: gain\r\nContent-Type: text/plain\r\nDate: ")
	httpLastLine  = getHTTPLastLine()
)

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

type EchoHandler struct {
	gain.DefaultEventHandler
}

func (h EchoHandler) OnRead(c gain.Conn, n int) {
	buf, _ := c.Next(n)
	_, _ = c.Write(buf)
}

type HTTPHandler struct {
	gain.DefaultEventHandler
}

func (h HTTPHandler) OnRead(conn gain.Conn, n int) {
	_, err := conn.Discard(n)
	if err != nil {
		log.Panic(err)
	}

	_, err = conn.Write(httpFirstLine)
	if err != nil {
		log.Panic(err)
	}

	_, err = conn.Write([]byte(NowTimeFormat()))
	if err != nil {
		log.Panic(err)
	}

	_, err = conn.Write(httpLastLine)
	if err != nil {
		log.Panic(err)
	}
}

func main() {
	runtime.GOMAXPROCS(goMaxProcs)

	go func() {
		server := &http.Server{
			Addr:              "0.0.0.0:6061",
			ReadHeaderTimeout: pprofReadHeaderTimeout,
		}
		log.Println(server.ListenAndServe())
	}()

	var server gain.Server
	app := &cli.App{
		EnableBashCompletion: true,
		Flags:                flags,
		Name:                 "cli",
		Usage:                "Gain TCP/UDP server",
		Action: func(*cli.Context) error {
			fmt.Printf("Configuration: %+v \n", config)
			fmt.Printf("Starting Gain %s Server...\n", config.network)
			if config.handler == "echo" {
				server = gain.NewServer(EchoHandler{}, config.Config)
			} else if config.handler == "http" {
				server = gain.NewServer(HTTPHandler{}, config.Config)
			}

			return server.Start(fmt.Sprintf("%s://0.0.0.0:%d", config.network, port))
		},
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		fmt.Println("Server closing...")
		server.Shutdown()
	}()

	if err := app.Run(os.Args); err != nil {
		log.Panic(err)
	}
}

func init() {
	now.Store(nowTimeFormat())

	go ticktock()
}
