package gain

import (
	"syscall"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/rs/zerolog"
)

type connCloser struct {
	ring   *iouring.Ring
	logger zerolog.Logger
}

func (c *connCloser) addCloseRequest(fd int) (*iouring.SubmissionQueueEntry, error) {
	entry, err := c.ring.GetSQE()
	if err != nil {
		return nil, err
	}
	entry.PrepareClose(fd)
	entry.UserData = closeConnFlag | uint64(fd)
	return entry, nil
}

func (c *connCloser) addCloseConnRequest(conn *connection) (*iouring.SubmissionQueueEntry, error) {
	entry, err := c.addCloseRequest(conn.fd)
	if err != nil {
		return nil, err
	}
	conn.state = connClose
	return entry, nil
}

func (c *connCloser) syscallShutdownSocket(fileDescriptor int) error {
	return syscall.Shutdown(fileDescriptor, syscall.SHUT_RDWR)
}

func newConnCloser(ring *iouring.Ring, logger zerolog.Logger) *connCloser {
	return &connCloser{
		ring:   ring,
		logger: logger,
	}
}
