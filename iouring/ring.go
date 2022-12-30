package iouring

const (
	SQNeedWakeup uint32 = 1 << iota
	SQCQOverflow
	SQTaskrun
)

const (
	IntFlagRegRing uint8 = 1
)

type SubmissionQueue struct {
	buffer    []byte
	sqeBuffer []byte
	ringSize  uint64

	head        *uint32
	tail        *uint32
	ringMask    *uint32
	ringEntries *uint32
	flags       *uint32
	dropped     *uint32
	array       *uint32

	sqeTail uint32
	sqeHead uint32
}

type CompletionQueue struct {
	buffer   []byte
	ringSize uint64

	head        *uint32
	tail        *uint32
	ringMask    *uint32
	ringEntries *uint32
	overflow    *uint32
	flags       uintptr

	cqeBuff *CompletionQueueEvent
}

type Ring struct {
	sqRing      *SubmissionQueue
	cqRing      *CompletionQueue
	flags       uint32
	fd          int
	features    uint32
	enterRingFd int
	intFlags    uint8
	params      *Params

	exited bool
}

func (ring *Ring) Fd() int {
	return ring.fd
}

func newRing() *Ring {
	return &Ring{
		params: &Params{},
		sqRing: &SubmissionQueue{},
		cqRing: &CompletionQueue{},
	}
}

func CreateRing() (*Ring, error) {
	ring := newRing()
	var flags uint32
	err := ring.QueueInit(defaultMaxQueue, flags)
	if err != nil {
		return nil, err
	}
	return ring, nil
}
