package iouring_test

import (
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/pawelgaczynski/gain/iouring"
	. "github.com/stretchr/testify/require"
)

func TestSubmitAndWait(t *testing.T) {
	ring, err := iouring.CreateRing()
	NoError(t, err)
	defer ring.Close()

	cqeBuff := make([]*iouring.CompletionQueueEvent, 16)

	cnt := ring.PeekBatchCQE(cqeBuff)
	Equal(t, 0, cnt)

	NoError(t, queueNOPs(t, ring, 4, 0))

	timespec := syscall.NsecToTimespec((time.Millisecond * 100).Nanoseconds())
	_, err = ring.SubmitAndWaitTimeout(10, &timespec)
	runtime.KeepAlive(timespec)

	NoError(t, err)
}

func TestSubmitAndWaitNilTimeout(t *testing.T) {
	ring, err := iouring.CreateRing()
	NoError(t, err)
	defer ring.Close()

	cqeBuff := make([]*iouring.CompletionQueueEvent, 16)

	cnt := ring.PeekBatchCQE(cqeBuff)
	Equal(t, 0, cnt)

	NoError(t, queueNOPs(t, ring, 4, 0))

	_, err = ring.SubmitAndWaitTimeout(1, nil)
	NoError(t, err)
}
