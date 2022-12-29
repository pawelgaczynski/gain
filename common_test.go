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

const testPort = 9876

type testServerConfig struct {
	numberOfClients       int
	numberOfWorkers       int
	lockOSThread          bool
	asyncHandler          bool
	goroutinePool         bool
	batchSubmitter        bool
	prefillConnectionPool bool
	waitForDialAllClients bool
}

type TestServerHandler struct {
	onOpenCallback func(int)
	onRWCallback   func(int)
}

func (h *TestServerHandler) OnOpen(fd int) {
	if h.onOpenCallback != nil {
		h.onOpenCallback(fd)
	}
}
func (h *TestServerHandler) OnClose(fd int) {}
func (h *TestServerHandler) OnData(c gain.Conn) error {
	buffer := make([]byte, 128)
	_, err := c.Read(buffer)
	if err != nil {
		return err
	}
	_, err = c.Write([]byte("TESTpayload12345"))
	if err != nil {
		return err
	}
	return nil
}

func (h *TestServerHandler) AfterWrite(c gain.Conn) {
	if h.onRWCallback != nil {
		h.onRWCallback(c.Fd())
	}
}

func dialClient(t *testing.T, clientConnChan chan net.Conn) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", testPort), time.Second)
	Nil(t, err)
	NotNil(t, conn)
	clientConnChan <- conn
}

func dialClientRW(t *testing.T, clientConnChan chan net.Conn) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", testPort), 2*time.Second)
	Nil(t, err)
	NotNil(t, conn)
	err = conn.SetDeadline(time.Now().Add(time.Millisecond * 100))
	Nil(t, err)
	n, err := conn.Write([]byte("testdata1234567890"))
	Nil(t, err)
	Equal(t, 18, n)
	var buffer [16]byte
	n, err = conn.Read(buffer[:])
	Nil(t, err)
	Equal(t, 16, n)
	Equal(t, "TESTpayload12345", string(buffer[:]))
	clientConnChan <- conn
}

func testServer(t *testing.T, testConfig testServerConfig, isReactor bool) {
	opts := []gain.EngineOption{
		gain.WithLoggerLevel(zerolog.DebugLevel),
		gain.WithMaxConn(uint(testConfig.numberOfClients)),
		gain.WithBufferSize(uint(128)),
		gain.WithPort(testPort),
		gain.WithAsyncHandler(testConfig.asyncHandler),
		gain.WithGoroutinePool(testConfig.goroutinePool),
		gain.WithLockOSThread(testConfig.lockOSThread),
		gain.WithWorkers(testConfig.numberOfWorkers),
		gain.WithBatchSubmitter(testConfig.batchSubmitter),
		gain.WithPrefillConnectionPool(testConfig.prefillConnectionPool),
		gain.WithCBPF(false),
		gain.WithSharding(!isReactor),
	}

	opts = append(opts, []gain.EngineOption{}...)
	config := gain.NewConfig(opts...)

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	handler := new(TestServerHandler)
	server := gain.NewServer(config, handler)
	server.SetStartListener(func() {
		waitGroup.Done()
	})
	defer func() {
		server.Close()
	}()

	go func() {
		err := server.Start()
		if err != nil {
			log.Panic(err)
		}
		Nil(t, err)
	}()

	clientConnChan := make(chan net.Conn, testConfig.numberOfClients)
	waitGroup.Wait()
	if testConfig.waitForDialAllClients {
		var clientConnectWG = new(sync.WaitGroup)
		clientConnectWG.Add(testConfig.numberOfClients)
		handler.onOpenCallback = func(fd int) {
			clientConnectWG.Done()
		}
		for i := 0; i < testConfig.numberOfClients; i++ {
			go dialClient(t, clientConnChan)
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
		var clientConnectWG = new(sync.WaitGroup)
		clientConnectWG.Add(testConfig.numberOfClients)
		var clientRWWG = new(sync.WaitGroup)
		clientRWWG.Add(testConfig.numberOfClients)
		handler.onOpenCallback = func(fd int) {
			clientConnectWG.Done()
		}
		handler.onRWCallback = func(fd int) {
			clientRWWG.Done()
		}
		for i := 0; i < testConfig.numberOfClients; i++ {
			go func(numSec int) {
				dialClientRW(t, clientConnChan)
			}(i)
		}
		clientConnectWG.Wait()
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
