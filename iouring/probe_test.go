// Copyright (c) 2023 Paweł Gaczyński
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iouring

import (
	"testing"

	. "github.com/stretchr/testify/require"
)

func TestCheckAvailableFeatures(t *testing.T) {
	availableFeatures, err := CheckAvailableFeatures()
	Nil(t, err)
	Contains(t, availableFeatures, "IORING_OP_NOP is supported")
	Contains(t, availableFeatures, "IORING_OP_READV is supported")
	Contains(t, availableFeatures, "IORING_OP_WRITEV is supported")
	Contains(t, availableFeatures, "IORING_OP_FSYNC is supported")
	Contains(t, availableFeatures, "IORING_OP_READ_FIXED is supported")
	Contains(t, availableFeatures, "IORING_OP_WRITE_FIXED is supported")
	Contains(t, availableFeatures, "IORING_OP_POLL_ADD is supported")
	Contains(t, availableFeatures, "IORING_OP_POLL_REMOVE is supported")
	Contains(t, availableFeatures, "IORING_OP_SYNC_FILE_RANGE is supported")
	Contains(t, availableFeatures, "IORING_OP_SENDMSG is supported")
	Contains(t, availableFeatures, "IORING_OP_RECVMSG is supported")
	Contains(t, availableFeatures, "IORING_OP_TIMEOUT is supported")
	Contains(t, availableFeatures, "IORING_OP_TIMEOUT_REMOVE is supported")
	Contains(t, availableFeatures, "IORING_OP_ACCEPT is supported")
	Contains(t, availableFeatures, "IORING_OP_ASYNC_CANCEL is supported")
	Contains(t, availableFeatures, "IORING_OP_LINK_TIMEOUT is supported")
	Contains(t, availableFeatures, "IORING_OP_CONNECT is supported")
	Contains(t, availableFeatures, "IORING_OP_FALLOCATE is supported")
	Contains(t, availableFeatures, "IORING_OP_OPENAT is supported")
	Contains(t, availableFeatures, "IORING_OP_CLOSE is supported")
	Contains(t, availableFeatures, "IORING_OP_FILES_UPDATE is supported")
	Contains(t, availableFeatures, "IORING_OP_STATX is supported")
	Contains(t, availableFeatures, "IORING_OP_READ is supported")
	Contains(t, availableFeatures, "IORING_OP_WRITE is supported")
	Contains(t, availableFeatures, "IORING_OP_FADVISE is supported")
	Contains(t, availableFeatures, "IORING_OP_MADVISE is supported")
	Contains(t, availableFeatures, "IORING_OP_SEND is supported")
	Contains(t, availableFeatures, "IORING_OP_RECV is supported")
}

func TestIsOpSupported(t *testing.T) {
	for _, opCode := range []uint8{
		OpNop,
		OpReadv,
		OpWritev,
		OpFsync,
		OpReadFixed,
		OpWriteFixed,
		OpPollAdd,
		OpPollRemove,
		OpSyncFileRange,
		OpSendmsg,
		OpRecvmsg,
		OpTimeout,
		OpTimeoutRemove,
		OpAccept,
		OpAsyncCancel,
		OpLinkTimeout,
		OpConnect,
		OpFallocate,
		OpOpenat,
		OpClose,
		OpFilesUpdate,
		OpStatx,
		OpRead,
		OpWrite,
		OpFadvise,
		OpMadvise,
		OpSend,
		OpRecv,
	} {
		supported, err := IsOpSupported(opCode)
		Nil(t, err)
		Equal(t, true, supported)
	}
}
