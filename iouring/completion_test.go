package iouring_test

import (
	"testing"

	"github.com/pawelgaczynski/gain/iouring"
	"github.com/stretchr/testify/assert"
	. "github.com/stretchr/testify/require"
)

func queueNOPs(t *testing.T, ring *iouring.Ring, number int, offset int) error {
	for i := 0; i < number; i++ {
		entry, err := ring.GetSQE()
		if err != nil {
			return err
		}

		entry.PrepareNop()
		entry.UserData = uint64(i + offset)
	}
	submitted, err := ring.Submit()
	Equal(t, int(submitted), number)
	return err
}

func TestPeekBatchCQE(t *testing.T) {
	ring, err := iouring.CreateRing()
	NoError(t, err)
	defer ring.Close()

	cqeBuff := make([]*iouring.CompletionQueueEvent, 16)

	cnt := ring.PeekBatchCQE(cqeBuff)
	Equal(t, 0, cnt)

	NoError(t, queueNOPs(t, ring, 4, 0))

	cnt = ring.PeekBatchCQE(cqeBuff)
	Equal(t, 4, cnt)
	for i := 0; i < 4; i++ {
		assert.Equal(t, uint64(i), cqeBuff[i].UserData())
	}

	NoError(t, queueNOPs(t, ring, 4, 4))

	ring.CQAdvance(uint32(4))
	cnt = ring.PeekBatchCQE(cqeBuff)
	Equal(t, 4, cnt)
	for i := 0; i < 4; i++ {
		Equal(t, uint64(i+4), cqeBuff[i].UserData())
	}

	ring.CQAdvance(uint32(4))
}
