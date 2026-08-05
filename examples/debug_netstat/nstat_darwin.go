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

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

const (
	netStatControlName = "com.apple.network.statistics"

	nstatMsgTypeAddAllSrcs uint32 = 1002
	nstatMsgTypeQuerySrc   uint32 = 1004

	nstatMsgTypeSuccess   uint32 = 0
	nstatMsgTypeError     uint32 = 1
	nstatMsgTypeSrcAdded  uint32 = 10001
	nstatMsgTypeSrcCounts uint32 = 10004

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
	Hdr    nstatMsgHdr
	SrcRef uint64
}

type nstatMsgQuerySrcReq struct {
	Hdr    nstatMsgHdr
	SrcRef uint64
}

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

type nstatMsgErr struct {
	Hdr   nstatMsgHdr
	Error uint32
}

func runPlatformDiagnostics(pid int) {
	fmt.Println("\n--- 4. macOS nstat Control Socket Provider Diagnostics ---")

	fd, err := openNstatSocket()
	if err != nil {
		fmt.Printf("Failed to open nstat socket: %v\n", err)
		return
	}
	defer unix.Close(fd)

	providers := map[uint32]string{
		1: "NSTAT_PROVIDER_ROUTE (1)",
		2: "NSTAT_PROVIDER_TCP_KERNEL (2)",
		3: "NSTAT_PROVIDER_TCP_USERLAND (3)",
		4: "NSTAT_PROVIDER_UDP_KERNEL (4)",
		5: "NSTAT_PROVIDER_UDP_USERLAND (5)",
	}

	for provID := uint32(1); provID <= 5; provID++ {
		provName := providers[provID]
		refs, err := nstatCollectSrcRefs(fd, provID, int32(pid))
		if err != nil {
			fmt.Printf("  Provider %-32s -> Error: %v\n", provName, err)
			continue
		}

		var totalRx, totalTx uint64
		activeSources := 0
		for _, ref := range refs {
			rx, tx, err := nstatQueryCounts(fd, ref)
			if err != nil {
				continue
			}
			totalRx += rx
			totalTx += tx
			if rx > 0 || tx > 0 {
				activeSources++
			}
		}

		fmt.Printf("  Provider %-32s -> %3d sources (%3d active), rxBytes=%-12d, txBytes=%-12d\n",
			provName, len(refs), activeSources, totalRx, totalTx)
	}

	fmt.Println("\n--- Diagnosis / Technical Explanation ---")
	fmt.Println("1. Standard Go `net` / `net/http` connections create kernel BSD sockets (`AF_INET`/`AF_INET6`),")
	fmt.Println("   which are registered under TCP_KERNEL (provider 2) & UDP_KERNEL (provider 4).")
	fmt.Println("2. TCP_USERLAND (3) and UDP_USERLAND (5) only track Apple Network.framework / userland network stack sockets,")
	fmt.Println("   so they return 0 sources for standard Go process traffic.")
	fmt.Println("3. On macOS, NSTAT_FILTER_SPECIFIC_USER_BY_PID (0x01000000) is ignored by kernel providers (2 & 4),")
	fmt.Println("   causing kernel providers to return all system-wide sockets (attributing other processes' traffic).")
	fmt.Println("4. Hence, querying userland providers (3 & 5) results in network bytes = 0 for standard Go code.")
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

	tv := unix.NsecToTimeval(nstatReadTimeout.Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("nstat: SetsockoptTimeval: %w", err)
	}

	return fd, nil
}

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

	var refs []uint64
	buf := make([]byte, 4096)
	for {
		n, err := nstatRead(fd, buf)
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

	buf := make([]byte, 4096)
	for {
		n, err := nstatRead(fd, buf)
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

func nstatRead(fd int, buf []byte) (int, error) {
	n, err := unix.Read(fd, buf)
	if err != nil {
		return 0, fmt.Errorf("nstat: read: %w", err)
	}
	if n < 16 {
		return 0, fmt.Errorf("nstat: short read (%d bytes)", n)
	}
	return n, nil
}

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
