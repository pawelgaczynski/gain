package gain

import (
	"errors"
	"syscall"
	"time"

	"github.com/pawelgaczynski/gain/iouring"
)

var waitForArray = []uint32{
	1,
	32,
	64,
	96,
	128,
	256,
	384,
	512,
	768,
	1024,
	1536,
	2048,
	3072,
	4096,
	5120,
	6144,
	7168,
	8192,
	10240,
}

type batchSubmitter struct {
	ring            *iouring.Ring
	timeoutTimeSpec syscall.Timespec
	waitForIndex    uint32
	waitFor         uint32
}

func (s *batchSubmitter) submit() error {
	_, err := s.ring.SubmitAndWaitTimeout(s.waitFor, &s.timeoutTimeSpec)
	if errors.Is(err, iouring.ErrAgain) || errors.Is(err, iouring.ErrInterrupredSyscall) ||
		errors.Is(err, iouring.ErrTimerExpired) {
		if s.waitForIndex != 0 {
			s.waitForIndex--
			s.waitFor = waitForArray[s.waitForIndex]
		}
		return errSkippable
	}
	return err
}

func (s *batchSubmitter) advance(n uint32) {
	s.ring.CQAdvance(n)
	var i uint32
	var lenWaitForArray = uint32(len(waitForArray))
	for i = 1; i < lenWaitForArray; i++ {
		if waitForArray[i] > n {
			break
		}
		s.waitForIndex = i
	}
	s.waitFor = waitForArray[s.waitForIndex]
}

func newBatchSubmitter(ring *iouring.Ring) *batchSubmitter {
	s := &batchSubmitter{
		ring:            ring,
		timeoutTimeSpec: syscall.NsecToTimespec((time.Millisecond).Nanoseconds()),
	}
	s.waitFor = waitForArray[s.waitForIndex]
	return s
}
