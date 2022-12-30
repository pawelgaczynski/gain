package gain

import (
	"syscall"
)

type buffer struct {
	data   []byte
	iovecs []syscall.Iovec
}

func createBuffer(bufferSize uint) *buffer {
	bufferMem := make([]byte, bufferSize)
	iovec := syscall.Iovec{
		Base: &bufferMem[0],
		Len:  uint64(bufferSize),
	}
	buff := &buffer{
		data:   bufferMem,
		iovecs: []syscall.Iovec{iovec},
	}
	return buff
}

func createBuffers(bufferSize uint, maxConn uint) []*buffer {
	memSize := maxConn * bufferSize
	bufferMem := make([]byte, memSize)
	buffs := make([]*buffer, maxConn)
	for index := range buffs {
		startIndex := index * int(bufferSize)
		buff := bufferMem[startIndex : startIndex+int(bufferSize)]
		iovec := syscall.Iovec{
			Base: &buff[0],
			Len:  uint64(bufferSize),
		}
		buffs[index] = &buffer{
			data:   buff,
			iovecs: []syscall.Iovec{iovec},
		}
	}
	return buffs
}
