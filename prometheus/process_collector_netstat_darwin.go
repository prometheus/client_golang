// Copyright The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build darwin && !ios

package prometheus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// This file implements a client for the undocumented "com.apple.network.statistics"
// kernel control socket, which is what Apple's own `nettop`/`netstat` tools use to
// report per-process network byte counters. There is no public Apple API for this
// and no cgo or third-party dependency is used: the protocol is implemented directly
// on top of golang.org/x/sys/unix, using the same PF_SYSTEM/SYSPROTO_CONTROL socket
// mechanism as the utun driver. Struct layouts below mirror bsd/net/ntstat.h from
// Apple's XNU source (https://github.com/apple-oss-distributions/xnu/blob/main/bsd/net/ntstat.h).

const (
	netStatControlName = "com.apple.network.statistics"

	// Plain BSD sockets, which is what Go's net package creates, are tracked by the
	// kernel providers. The userland providers (3 and 5) track flows of the userland
	// networking stack. They are deliberately not queried, because Go sockets do not
	// show up there and their pid filtering could not be verified.
	nstatProviderTCPKernel uint32 = 2
	nstatProviderUDPKernel uint32 = 4

	nstatMsgTypeAddAllSrcs uint32 = 1002
	nstatMsgTypeQuerySrc   uint32 = 1004

	nstatMsgTypeSuccess   uint32 = 0
	nstatMsgTypeError     uint32 = 1
	nstatMsgTypeSrcAdded  uint32 = 10001
	nstatMsgTypeSrcCounts uint32 = 10004

	// NSTAT_FILTER_SPECIFIC_USER_BY_PID from bsd/net/ntstat.h. The filter is a 64-bit
	// mask. Older XNU headers defined this bit as 0x01000000, which current XNU uses
	// for the unrelated NSTAT_FILTER_CONN_HAS_NET_ACCESS and which therefore does not
	// select a pid at all. With the correct mask the kernel reports zero counts for
	// the sources of every other process.
	nstatFilterSpecificUserByPid uint64 = 0x100000000

	// Request contexts, echoed back by the kernel in the responses to that request.
	nstatContextAddAllSrcs uint64 = 1
	nstatContextQuerySrc   uint64 = 2

	nstatReadTimeout = 2 * time.Second

	// The kernel queues one datagram per source while enumerating them, and silently
	// drops whatever does not fit into the receive buffer. The default of 8 KiB holds
	// only ~190 sources, which is less than the number of sockets on a typical
	// machine, so the buffer has to be enlarged to not undercount.
	nstatRecvBufSize = 4 << 20

	// Large enough for any message (MAX_NSTAT_MSG_HDR_LENGTH in ntstat.h is 65532).
	nstatReadBufSize = 64 << 10

	nstatMsgHdrSize = 16
)

type nstatMsgHdr struct {
	Context uint64
	Type    uint32
	Length  uint16
	Flags   uint16
}

type nstatMsgAddAllSrcs struct {
	Hdr        nstatMsgHdr
	Filter     uint64
	Events     uint64
	Provider   uint32
	TargetPid  int32
	TargetUUID [16]byte
}

// nstatMsgSrcAdded mirrors the leading fields of struct nstat_msg_src_added.
// The trailing Provider and Reserved fields are present on the wire but unused
// here, so binary.Read simply leaves them unread.
type nstatMsgSrcAdded struct {
	Hdr    nstatMsgHdr
	SrcRef uint64
}

type nstatMsgQuerySrcReq struct {
	Hdr    nstatMsgHdr
	SrcRef uint64
}

// nstatCounts mirrors the leading fields of struct nstat_counts. The real struct
// has more trailing fields (retransmits, RTT estimates, etc.). Only the byte
// counters are needed, and binary.Read leaves the rest of the message unread, so
// trimming here avoids coupling to the full XNU layout.
type nstatCounts struct {
	RxPackets uint64
	RxBytes   uint64
	TxPackets uint64
	TxBytes   uint64
}

type nstatMsgSrcCounts struct {
	Hdr        nstatMsgHdr
	SrcRef     uint64
	EventFlags uint64
	Counts     nstatCounts
}

// nstatMsgErr mirrors the leading fields of struct nstat_msg_error. The trailing
// Reserved field is unused here.
type nstatMsgErr struct {
	Hdr   nstatMsgHdr
	Error uint32
}

// nstatClient is a connection to the network statistics control socket. It is not
// safe for concurrent use.
type nstatClient struct {
	fd  int
	buf []byte
}

func newNstatClient() (*nstatClient, error) {
	fd, err := unix.Socket(unix.AF_SYSTEM, unix.SOCK_DGRAM, 2 /* SYSPROTO_CONTROL */)
	if err != nil {
		return nil, fmt.Errorf("nstat: socket: %w", err)
	}

	ctlInfo := &unix.CtlInfo{}
	copy(ctlInfo.Name[:], netStatControlName)
	if err := unix.IoctlCtlInfo(fd, ctlInfo); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("nstat: IoctlCtlInfo: %w", err)
	}

	if err := unix.Connect(fd, &unix.SockaddrCtl{ID: ctlInfo.Id}); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("nstat: connect: %w", err)
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, nstatRecvBufSize); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("nstat: SO_RCVBUF: %w", err)
	}

	tv := unix.NsecToTimeval(nstatReadTimeout.Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("nstat: SO_RCVTIMEO: %w", err)
	}

	return &nstatClient{fd: fd, buf: make([]byte, nstatReadBufSize)}, nil
}

func (c *nstatClient) close() {
	unix.Close(c.fd)
}

// getNetworkBytes returns the total bytes received and sent over the network
// by the current process, summed across its TCP and UDP sockets.
//
// The result is a snapshot of the sockets that are alive at the time of the call.
// The kernel forgets a socket once it is closed, so the bytes transferred by
// short-lived sockets disappear from the total after they are closed.
func getNetworkBytes() (rxBytes, txBytes uint64, err error) {
	c, err := newNstatClient()
	if err != nil {
		return 0, 0, err
	}
	defer c.close()

	pid := int32(os.Getpid())
	for _, provider := range []uint32{nstatProviderTCPKernel, nstatProviderUDPKernel} {
		refs, err := c.subscribe(provider, pid)
		if err != nil {
			return 0, 0, fmt.Errorf("nstat: enumerating sources for provider %d: %w", provider, err)
		}
		for _, ref := range refs {
			rx, tx, err := c.queryCounts(ref)
			if err != nil {
				return 0, 0, fmt.Errorf("nstat: querying source %d of provider %d: %w", ref, provider, err)
			}
			rxBytes += rx
			txBytes += tx
		}
	}

	return rxBytes, txBytes, nil
}

// subscribe subscribes to all sources of the given provider and returns their
// references. The kernel replies with zero or more SRC_ADDED messages followed
// by a SUCCESS message carrying the context of the request, which marks the end
// of the enumeration. Counts of the sources not owned by pid are reported as zero.
func (c *nstatClient) subscribe(provider uint32, pid int32) ([]uint64, error) {
	req := nstatMsgAddAllSrcs{
		Hdr:       nstatMsgHdr{Context: nstatContextAddAllSrcs, Type: nstatMsgTypeAddAllSrcs},
		Filter:    nstatFilterSpecificUserByPid,
		Provider:  provider,
		TargetPid: pid,
	}
	if err := c.send(&req, &req.Hdr); err != nil {
		return nil, err
	}

	var (
		refs []uint64
		done bool
	)
	for !done {
		err := c.readMsgs(func(hdr nstatMsgHdr, msg []byte) error {
			switch hdr.Type {
			case nstatMsgTypeSrcAdded:
				var m nstatMsgSrcAdded
				if err := decodeNstatMsg(msg, &m); err != nil {
					return fmt.Errorf("decoding SRC_ADDED: %w", err)
				}
				refs = append(refs, m.SrcRef)
			case nstatMsgTypeSuccess:
				done = done || hdr.Context == nstatContextAddAllSrcs
			case nstatMsgTypeError:
				if hdr.Context == nstatContextAddAllSrcs {
					return nstatKernelError(msg)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return refs, nil
}

// queryCounts returns the byte counters of a single source. A source that has
// been closed since it was enumerated is not an error, it simply has no counters
// anymore.
func (c *nstatClient) queryCounts(srcref uint64) (rxBytes, txBytes uint64, err error) {
	req := nstatMsgQuerySrcReq{
		Hdr:    nstatMsgHdr{Context: nstatContextQuerySrc, Type: nstatMsgTypeQuerySrc},
		SrcRef: srcref,
	}
	if err := c.send(&req, &req.Hdr); err != nil {
		return 0, 0, err
	}

	done := false
	for !done {
		err := c.readMsgs(func(hdr nstatMsgHdr, msg []byte) error {
			switch hdr.Type {
			case nstatMsgTypeSrcCounts:
				var m nstatMsgSrcCounts
				if err := decodeNstatMsg(msg, &m); err != nil {
					return fmt.Errorf("decoding SRC_COUNTS: %w", err)
				}
				// A subscription also delivers unsolicited counts (e.g. when a
				// source is closed), which carry a different context and may
				// belong to another source.
				if hdr.Context == nstatContextQuerySrc && m.SrcRef == srcref {
					rxBytes, txBytes, done = m.Counts.RxBytes, m.Counts.TxBytes, true
				}
			case nstatMsgTypeError:
				if hdr.Context != nstatContextQuerySrc {
					return nil
				}
				err := nstatKernelError(msg)
				if !errors.Is(err, unix.ENOENT) {
					return err
				}
				done = true
			}
			return nil
		})
		if err != nil {
			return 0, 0, err
		}
	}
	return rxBytes, txBytes, nil
}

// send encodes msg and writes it to the socket after filling in the message length
// of hdr, which must point to the header embedded in msg.
func (c *nstatClient) send(msg any, hdr *nstatMsgHdr) error {
	hdr.Length = uint16(binary.Size(msg))
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, msg); err != nil {
		return fmt.Errorf("nstat: encoding request: %w", err)
	}
	if _, err := unix.Write(c.fd, buf.Bytes()); err != nil {
		return fmt.Errorf("nstat: write: %w", err)
	}
	return nil
}

// readMsgs reads one datagram from the control socket and calls fn for every
// message in it. SO_RCVTIMEO is set once on the socket, so a stalled kernel
// can't hang the collection forever.
func (c *nstatClient) readMsgs(fn func(hdr nstatMsgHdr, msg []byte) error) error {
	n, err := unix.Read(c.fd, c.buf)
	if err != nil {
		return fmt.Errorf("nstat: read: %w", err)
	}
	return forEachNstatMsg(c.buf[:n], fn)
}

// forEachNstatMsg calls fn for every message in a datagram. The kernel may pack
// several messages into a single datagram, each of them starting with a header
// whose Length covers the whole message (see the note on aggregate responses in
// ntstat.h).
func forEachNstatMsg(data []byte, fn func(hdr nstatMsgHdr, msg []byte) error) error {
	for len(data) > 0 {
		if len(data) < nstatMsgHdrSize {
			return fmt.Errorf("nstat: short message (%d bytes)", len(data))
		}
		var hdr nstatMsgHdr
		if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &hdr); err != nil {
			return fmt.Errorf("nstat: decoding header: %w", err)
		}
		msgLen := int(hdr.Length)
		if msgLen < nstatMsgHdrSize || msgLen > len(data) {
			// Not a valid length, treat the rest of the datagram as a single message.
			msgLen = len(data)
		}
		if err := fn(hdr, data[:msgLen]); err != nil {
			return err
		}
		data = data[msgLen:]
	}
	return nil
}

// decodeNstatMsg decodes the leading fields of msg into v. Trailing bytes of the
// message that v does not cover are ignored.
func decodeNstatMsg(msg []byte, v any) error {
	return binary.Read(bytes.NewReader(msg), binary.LittleEndian, v)
}

// nstatKernelError converts an ERROR message into a Go error wrapping the errno.
func nstatKernelError(msg []byte) error {
	var m nstatMsgErr
	if err := decodeNstatMsg(msg, &m); err != nil {
		return fmt.Errorf("nstat: decoding ERROR: %w", err)
	}
	return fmt.Errorf("nstat: kernel returned an error: %w", unix.Errno(m.Error))
}
