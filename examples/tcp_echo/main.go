package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/pawelgaczynski/gain"
	"github.com/pawelgaczynski/gain/logger"
	"github.com/rs/zerolog"
)

const (
	port            = 8080
	numberOfClients = 2
)

var testData = []byte("echo")

type EventHandler struct {
	server gain.Server

	logger zerolog.Logger

	overallBytesSent atomic.Uint64
}

func (e *EventHandler) OnStart(server gain.Server) {
	e.server = server
	e.logger = zerolog.New(os.Stdout).With().Logger().Level(zerolog.InfoLevel)
}

func (e *EventHandler) OnAccept(conn gain.Conn) {
	e.logger.Info().
		Int("active connections", e.server.ActiveConnections()).
		Str("remote address", conn.RemoteAddr().String()).
		Msg("New connection accepted")
}

func (e *EventHandler) OnRead(conn gain.Conn, n int) {
	e.logger.Info().
		Int("bytes", n).
		Str("remote address", conn.RemoteAddr().String()).
		Msg("Bytes received from remote peer")

	var (
		err    error
		buffer []byte
	)

	buffer, err = conn.Next(n)
	if err != nil {
		return
	}

	_, _ = conn.Write(buffer)
}

func (e *EventHandler) OnWrite(conn gain.Conn, n int) {
	e.overallBytesSent.Add(uint64(n))

	e.logger.Info().
		Int("bytes", n).
		Str("remote address", conn.RemoteAddr().String()).
		Msg("Bytes sent to remote peer")

	err := conn.Close()
	if err != nil {
		e.logger.Error().Err(err).Msg("Error during connection close")
	}
}

func (e *EventHandler) OnClose(conn gain.Conn, err error) {
	log := e.logger.Info().
		Str("remote address", conn.RemoteAddr().String())
	if err != nil {
		log.Err(err).Msg("Connection from remote peer closed")
	} else {
		log.Msg("Connection from remote peer closed by server")
	}

	if e.overallBytesSent.Load() >= uint64(len(testData)*numberOfClients) {
		e.server.AsyncShutdown()
	}
}

func runClients() {
	for i := 0; i < numberOfClients; i++ {
		go func() {
			time.Sleep(time.Second)

			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
			if err != nil {
				log.Panic(err)
			}

			n, err := conn.Write(testData)
			if err != nil {
				log.Panic()
			}

			if n != len(testData) {
				log.Panic()
			}

			buffer := make([]byte, len(testData))

			n, err = conn.Read(buffer)
			if err != nil {
				log.Panic()
			}

			if n != len(testData) {
				log.Panic()
			}
		}()
	}
}

func main() {
	runClients()

	err := gain.ListenAndServe(
		fmt.Sprintf("tcp://localhost:%d", port), &EventHandler{}, gain.WithLoggerLevel(logger.WarnLevel))
	if err != nil {
		log.Panic(err)
	}
}
