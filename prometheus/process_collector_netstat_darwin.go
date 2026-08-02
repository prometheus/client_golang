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
// Apple's XNU source (https://github.com/apple/darwin-xnu/blob/main/bsd/net/ntstat.h).

const (
	netStatControlName = "com.apple.network.statistics"

	nstatProviderTCPUserland uint32 = 3
	nstatProviderUDPUserland uint32 = 5

	nstatMsgTypeAddAllSrcs uint32 = 1002
	nstatMsgTypeRemSrc     uint32 = 1003
	nstatMsgTypeQuerySrc   uint32 = 1004

	nstatMsgTypeSuccess   uint32 = 0
	nstatMsgTypeError     uint32 = 1
	nstatMsgTypeSrcAdded  uint32 = 10001
	nstatMsgTypeSrcCounts uint32 = 10004

	// Restrict subscription to sources owned by a single pid, rather than
	// system-wide (which would require elevated privileges anyway).
	nstatFilterSpecificUserByPid uint64 = 0x01000000

	nstatReadTimeout = 2 * time.Second
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

type nstatMsgSrcAdded struct {
	Hdr      nstatMsgHdr
	SrcRef   uint64
	Provider uint32
	Reserved [4]byte
}

type nstatMsgQuerySrcReq struct {
	Hdr    nstatMsgHdr
	SrcRef uint64
}

// nstatCounts mirrors struct nstat_counts. Only the first four fields are used
// here; the rest are read (to consume the full wire message) but not exposed.
type nstatCounts struct {
	RxPackets         uint64
	RxBytes           uint64
	TxPackets         uint64
	TxBytes           uint64
	CellRxBytes       uint64
	CellTxBytes       uint64
	WifiRxBytes       uint64
	WifiTxBytes       uint64
	WiredRxBytes      uint64
	WiredTxBytes      uint64
	RxDuplicateBytes  uint32
	RxOutOfOrderBytes uint32
	TxRetransmit      uint32
	ConnectAttempts   uint32
	ConnectSuccesses  uint32
	MinRtt            uint32
	AvgRtt            uint32
	VarRtt            uint32
}

type nstatMsgSrcCounts struct {
	Hdr        nstatMsgHdr
	SrcRef     uint64
	EventFlags uint64
	Counts     nstatCounts
}

type nstatMsgErr struct {
	Hdr      nstatMsgHdr
	Error    uint32
	Reserved [4]byte
}

// getNetworkBytes returns the total bytes received and sent over the network
// by the current process, summed across its TCP and UDP sockets.
func getNetworkBytes() (rxBytes, txBytes uint64, err error) {
	fd, err := openNstatSocket()
	if err != nil {
		return 0, 0, err
	}
	defer unix.Close(fd)

	pid := int32(os.Getpid())
	for _, provider := range []uint32{nstatProviderTCPUserland, nstatProviderUDPUserland} {
		refs, err := nstatCollectSrcRefs(fd, provider, pid)
		if err != nil {
			return 0, 0, fmt.Errorf("nstat: enumerating sources for provider %d: %w", provider, err)
		}
		for _, ref := range refs {
			rx, tx, err := nstatQueryCounts(fd, ref)
			if err != nil {
				// The source may have been torn down between enumeration and
				// query (e.g. a connection just closed); skip it rather than
				// failing the whole collection.
				continue
			}
			rxBytes += rx
			txBytes += tx
		}
	}

	return rxBytes, txBytes, nil
}

func openNstatSocket() (int, error) {
	fd, err := unix.Socket(unix.AF_SYSTEM, unix.SOCK_DGRAM, 2 /* SYSPROTO_CONTROL */)
	if err != nil {
		return -1, fmt.Errorf("nstat: socket: %w", err)
	}

	ctlInfo := &unix.CtlInfo{}
	copy(ctlInfo.Name[:], netStatControlName)
	if err := unix.IoctlCtlInfo(fd, ctlInfo); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("nstat: IoctlCtlInfo: %w", err)
	}

	if err := unix.Connect(fd, &unix.SockaddrCtl{ID: ctlInfo.Id}); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("nstat: connect: %w", err)
	}

	return fd, nil
}

// nstatCollectSrcRefs subscribes to all sources of the given provider owned by
// pid, and returns the srcrefs the kernel reports. The kernel replies with zero
// or more SRC_ADDED messages followed by a SUCCESS message carrying the same
// context, which marks the end of enumeration.
func nstatCollectSrcRefs(fd int, provider uint32, pid int32) ([]uint64, error) {
	const ctx = 1

	req := nstatMsgAddAllSrcs{
		Hdr:       nstatMsgHdr{Context: ctx, Type: nstatMsgTypeAddAllSrcs},
		Filter:    nstatFilterSpecificUserByPid,
		Provider:  provider,
		TargetPid: pid,
	}
	req.Hdr.Length = uint16(binary.Size(req))
	if err := nstatSend(fd, req); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(nstatReadTimeout)
	var refs []uint64
	buf := make([]byte, 4096)
	for {
		n, err := nstatRead(fd, buf, deadline)
		if err != nil {
			return nil, err
		}

		hdr, err := nstatReadHdr(buf[:n])
		if err != nil {
			return nil, err
		}

		switch hdr.Type {
		case nstatMsgTypeSrcAdded:
			var m nstatMsgSrcAdded
			if err := binary.Read(bytes.NewReader(buf[:n]), binary.LittleEndian, &m); err != nil {
				return nil, fmt.Errorf("nstat: decoding SRC_ADDED: %w", err)
			}
			refs = append(refs, m.SrcRef)
		case nstatMsgTypeSuccess:
			if hdr.Context == ctx {
				return refs, nil
			}
		case nstatMsgTypeError:
			nErr, err := nstatReadErr(buf[:n])
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("kernel returned errno %d", nErr)
		}
	}
}

func nstatQueryCounts(fd int, srcref uint64) (rxBytes, txBytes uint64, err error) {
	const ctx = 2

	req := nstatMsgQuerySrcReq{
		Hdr:    nstatMsgHdr{Context: ctx, Type: nstatMsgTypeQuerySrc},
		SrcRef: srcref,
	}
	req.Hdr.Length = uint16(binary.Size(req))
	if err := nstatSend(fd, req); err != nil {
		return 0, 0, err
	}

	deadline := time.Now().Add(nstatReadTimeout)
	buf := make([]byte, 4096)
	for {
		n, err := nstatRead(fd, buf, deadline)
		if err != nil {
			return 0, 0, err
		}

		hdr, err := nstatReadHdr(buf[:n])
		if err != nil {
			return 0, 0, err
		}

		switch hdr.Type {
		case nstatMsgTypeSrcCounts:
			var m nstatMsgSrcCounts
			if err := binary.Read(bytes.NewReader(buf[:n]), binary.LittleEndian, &m); err != nil {
				return 0, 0, fmt.Errorf("nstat: decoding SRC_COUNTS: %w", err)
			}
			return m.Counts.RxBytes, m.Counts.TxBytes, nil
		case nstatMsgTypeError:
			nErr, err := nstatReadErr(buf[:n])
			if err != nil {
				return 0, 0, err
			}
			return 0, 0, fmt.Errorf("kernel returned errno %d", nErr)
		}
	}
}

func nstatSend(fd int, msg any) error {
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, msg); err != nil {
		return fmt.Errorf("nstat: encoding request: %w", err)
	}
	if _, err := unix.Write(fd, buf.Bytes()); err != nil {
		return fmt.Errorf("nstat: write: %w", err)
	}
	return nil
}

// nstatRead reads one datagram from the control socket, applying deadline as a
// per-call receive timeout.
func nstatRead(fd int, buf []byte, deadline time.Time) (int, error) {
	d := time.Until(deadline)
	if d < 0 {
		d = 0
	}
	tv := unix.NsecToTimeval(d.Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		return 0, fmt.Errorf("nstat: SetsockoptTimeval: %w", err)
	}
	n, err := unix.Read(fd, buf)
	if err != nil {
		return 0, fmt.Errorf("nstat: read: %w", err)
	}
	if n < int(unsafeSizeofNstatMsgHdr) {
		return 0, fmt.Errorf("nstat: short read (%d bytes)", n)
	}
	return n, nil
}

const unsafeSizeofNstatMsgHdr = 16

func nstatReadHdr(buf []byte) (nstatMsgHdr, error) {
	var hdr nstatMsgHdr
	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, &hdr); err != nil {
		return hdr, fmt.Errorf("nstat: decoding header: %w", err)
	}
	return hdr, nil
}

func nstatReadErr(buf []byte) (uint32, error) {
	var m nstatMsgErr
	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, &m); err != nil {
		return 0, fmt.Errorf("nstat: decoding ERROR: %w", err)
	}
	return m.Error, nil
}
