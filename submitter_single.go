package gain

import (
	"errors"
	"syscall"
	"time"

	"github.com/pawelgaczynski/gain/iouring"
)

type singleSubmitter struct {
	ring            *iouring.Ring
	timeoutTimeSpec syscall.Timespec
}

func (s *singleSubmitter) submit() error {
	_, err := s.ring.SubmitAndWaitTimeout(1, &s.timeoutTimeSpec)
	if errors.Is(err, iouring.ErrAgain) || errors.Is(err, iouring.ErrInterrupredSyscall) ||
		errors.Is(err, iouring.ErrTimerExpired) {
		return errSkippable
	}
	return err
}

func (s *singleSubmitter) advance(n uint32) {
	s.ring.CQAdvance(n)
}

func newSingleSubmitter(ring *iouring.Ring) *singleSubmitter {
	return &singleSubmitter{
		ring:            ring,
		timeoutTimeSpec: syscall.NsecToTimespec((time.Millisecond).Nanoseconds()),
	}
}
