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

const (
	OpNop uint8 = iota
	OpReadv
	OpWritev
	OpFsync
	OpReadFixed
	OpWriteFixed
	OpPollAdd
	OpPollRemove
	OpSyncFileRange
	OpSendmsg
	OpRecvmsg
	OpTimeout
	OpTimeoutRemove
	OpAccept
	OpAsyncCancel
	OpLinkTimeout
	OpConnect
	OpFallocate
	OpOpenat
	OpClose
	OpFilesUpdate
	OpStatx
	OpRead
	OpWrite
	OpFadvise
	OpMadvise
	OpSend
	OpRecv
	OpOpenat2
	OpEpollCtl
	OpSplice
	OpProvideBuffers
	OpRemoveBuffers
	OpTee
	OpShutdown
	OpRenameat
	OpUnlinkat
	OpMkdirat
	OpSymlinkat
	OpLinkat
	OpMsgRing
	OpFsetxattr
	OpSetxattr
	OpFgetxattr
	OpGetxattr
	OpSocket
	OpUringCmd
	OpSendZC
	OpSendMsgZC

	OpLast
)

var opCodesMap = map[uint8]string{
	OpNop:            "IORING_OP_NOP",
	OpReadv:          "IORING_OP_READV",
	OpWritev:         "IORING_OP_WRITEV",
	OpFsync:          "IORING_OP_FSYNC",
	OpReadFixed:      "IORING_OP_READ_FIXED",
	OpWriteFixed:     "IORING_OP_WRITE_FIXED",
	OpPollAdd:        "IORING_OP_POLL_ADD",
	OpPollRemove:     "IORING_OP_POLL_REMOVE",
	OpSyncFileRange:  "IORING_OP_SYNC_FILE_RANGE",
	OpSendmsg:        "IORING_OP_SENDMSG",
	OpRecvmsg:        "IORING_OP_RECVMSG",
	OpTimeout:        "IORING_OP_TIMEOUT",
	OpTimeoutRemove:  "IORING_OP_TIMEOUT_REMOVE",
	OpAccept:         "IORING_OP_ACCEPT",
	OpAsyncCancel:    "IORING_OP_ASYNC_CANCEL",
	OpLinkTimeout:    "IORING_OP_LINK_TIMEOUT",
	OpConnect:        "IORING_OP_CONNECT",
	OpFallocate:      "IORING_OP_FALLOCATE",
	OpOpenat:         "IORING_OP_OPENAT",
	OpClose:          "IORING_OP_CLOSE",
	OpFilesUpdate:    "IORING_OP_FILES_UPDATE",
	OpStatx:          "IORING_OP_STATX",
	OpRead:           "IORING_OP_READ",
	OpWrite:          "IORING_OP_WRITE",
	OpFadvise:        "IORING_OP_FADVISE",
	OpMadvise:        "IORING_OP_MADVISE",
	OpSend:           "IORING_OP_SEND",
	OpRecv:           "IORING_OP_RECV",
	OpOpenat2:        "IORING_OP_OPENAT2",
	OpEpollCtl:       "IORING_OP_EPOLL_CTL",
	OpSplice:         "IORING_OP_SPLICE",
	OpProvideBuffers: "IORING_OP_PROVIDE_BUFFERS",
	OpRemoveBuffers:  "IORING_OP_REMOVE_BUFFERS",
	OpTee:            "IORING_OP_TEE",
	OpShutdown:       "IORING_OP_SHUTDOWN",
	OpRenameat:       "IORING_OP_RENAMEAT",
	OpUnlinkat:       "IORING_OP_UNLINKAT",
	OpMkdirat:        "IORING_OP_MKDIRAT",
	OpSymlinkat:      "IORING_OP_SYMLINKAT",
	OpLinkat:         "IORING_OP_LINKAT",
	OpMsgRing:        "IORING_OP_MSG_RING",
	OpFsetxattr:      "IORING_OP_FSETXATTR",
	OpSetxattr:       "IORING_OP_SETXATTR",
	OpFgetxattr:      "IORING_OP_FGETXATTR",
	OpGetxattr:       "IORING_OP_GETXATTR",
	OpSocket:         "IORING_OP_SOCKET",
	OpUringCmd:       "IORING_OP_URING_CMD",
	OpSendZC:         "IORING_OP_SEND_ZC",
	OpSendMsgZC:      "IORING_OP_SENDMSG_ZC",
}

const (
	RestrictionRegisterOp uint32 = iota
	RestrictionSQEOp
	RestrictionSQEFlagsAllowed
	RestrictionSQEFlagsRequired

	RestrictionLast
)
