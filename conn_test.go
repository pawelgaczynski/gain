package gain

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
	"time"

	. "github.com/stretchr/testify/require"
)

func TestConnectionStateString(t *testing.T) {
	Equal(t, "invalid", connInvalid.String())
	Equal(t, "accept", connAccept.String())
	Equal(t, "read", connRead.String())
	Equal(t, "write", connWrite.String())
	Equal(t, "close", connClose.String())
}

func TestConnModeString(t *testing.T) {
	Equal(t, "kernelSpace", connModeString(kernelSpace))
	Equal(t, "userSpace", connModeString(userSpace))
	Equal(t, "invalid", connModeString(2))
}

func TestUserOpAllowed(t *testing.T) {
	conn := newConnection()

	NotNil(t, conn.userOpAllowed("userOp"))

	conn.setKernelSpace()
	NotNil(t, conn.userOpAllowed("userOp"))

	conn.setUserSpace()
	Nil(t, conn.userOpAllowed("userOp"))

	conn.Close()
	NotNil(t, conn.userOpAllowed("userOp"))
}

func TestMethodsUserOpNotAllowed(t *testing.T) {
	conn := newConnection()

	Equal(t, "op is not available in mode, op: setReadBuffer, mode: kernelSpace", conn.SetReadBuffer(1024).Error())

	Equal(t, "op is not available in mode, op: setWriteBuffer, mode: kernelSpace", conn.SetWriteBuffer(1024).Error())

	Equal(t, "op is not available in mode, op: setLinger, mode: kernelSpace", conn.SetLinger(0).Error())

	Equal(t, "op is not available in mode, op: setNoDelay, mode: kernelSpace", conn.SetNoDelay(true).Error())

	Equal(t, "op is not available in mode, op: setKeepAlivePeriod, mode: kernelSpace",
		conn.SetKeepAlivePeriod(time.Second).Error())

	_, err := conn.Next(-1)
	Equal(t, "op is not available in mode, op: next, mode: kernelSpace", err.Error())

	_, err = conn.Discard(1024)
	Equal(t, "op is not available in mode, op: discard, mode: kernelSpace", err.Error())

	_, err = conn.Peek(1024)
	Equal(t, "op is not available in mode, op: peek, mode: kernelSpace", err.Error())

	data := make([]byte, 1024)
	_, err = rand.Read(data)
	NoError(t, err)
	reader := bytes.NewReader(data)
	_, err = conn.ReadFrom(reader)
	Equal(t, "op is not available in mode, op: readFrom, mode: kernelSpace", err.Error())

	buf := bytes.NewBuffer(nil)
	_, err = conn.WriteTo(buf)
	Equal(t, "op is not available in mode, op: writeTo, mode: kernelSpace", err.Error())

	_, err = conn.Read(make([]byte, 1024))
	Equal(t, "op is not available in mode, op: read, mode: kernelSpace", err.Error())

	_, err = conn.Write(make([]byte, 1024))
	Equal(t, "op is not available in mode, op: write, mode: kernelSpace", err.Error())
}

func TestConnectionCtx(t *testing.T) {
	conn := newConnection()

	var ctx context.Context

	conn.SetContext(ctx)
	Equal(t, ctx, conn.Context())
}

func TestConnectionReadFrom(t *testing.T) {
	conn := newConnection()
	conn.setUserSpace()

	data := make([]byte, 1024)
	_, err := rand.Read(data)
	NoError(t, err)
	reader := bytes.NewReader(data)
	n, err := conn.ReadFrom(reader)
	Nil(t, err)
	Equal(t, int64(1024), n)
	Equal(t, 1024, conn.OutboundBuffered())
}

func TestConnectionWriteTo(t *testing.T) {
	conn := newConnection()
	conn.setUserSpace()

	data := make([]byte, 1024)
	n, err := rand.Read(data)
	NoError(t, err)
	Equal(t, 1024, n)

	n, err = conn.inboundBuffer.Write(data)
	NoError(t, err)
	Equal(t, 1024, n)

	buffer := make([]byte, 1024)
	buf := bytes.NewBuffer(buffer)
	nBytes, err := conn.WriteTo(buf)
	NoError(t, err)
	Equal(t, int64(1024), nBytes)
}
