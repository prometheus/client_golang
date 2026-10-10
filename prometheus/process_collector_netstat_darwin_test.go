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
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const (
	// Payload size moved over loopback by the tests below.
	netstatTestBytes = 4 << 20

	netstatHelperEnv = "PROMETHEUS_TEST_NETSTAT_HELPER"
)

// networkBytesOrSkip returns the network byte counters of the current process,
// skipping the test if the statistics socket is not available (e.g. in a sandbox).
func networkBytesOrSkip(t *testing.T) (rx, tx uint64) {
	t.Helper()

	rx, tx, err := getNetworkBytes()
	if err != nil {
		t.Skipf("Network statistics are not available: %v.", err)
	}
	return rx, tx
}

// transferOverLoopback sends n bytes through a new loopback TCP connection and
// returns both ends of it, which are kept open for the caller to close.
func transferOverLoopback(t *testing.T, n int) (client, server net.Conn) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			close(accepted)
			return
		}
		accepted <- c
	}()

	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	// Reset instead of lingering in TIME_WAIT on close, otherwise repeated runs
	// exhaust the ephemeral ports.
	if tc, ok := client.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	server = <-accepted
	if server == nil {
		t.Fatal("Accepting the loopback connection failed.")
	}

	go func() { _, _ = client.Write(make([]byte, n)) }()
	if _, err := io.CopyN(io.Discard, server, int64(n)); err != nil {
		t.Fatal(err)
	}
	return client, server
}

func TestDarwinNetworkBytes(t *testing.T) {
	rxBefore, txBefore := networkBytesOrSkip(t)

	client, server := transferOverLoopback(t, netstatTestBytes)
	defer client.Close()
	defer server.Close()

	rxAfter, txAfter := networkBytesOrSkip(t)

	// The upper bound only guards against counting more than this process did,
	// the exact numbers include the protocol overhead of the kernel.
	for name, delta := range map[string]int64{
		"received":    int64(rxAfter) - int64(rxBefore),
		"transmitted": int64(txAfter) - int64(txBefore),
	} {
		if delta < netstatTestBytes || delta > 4*netstatTestBytes {
			t.Errorf("Bytes %s changed by %d, want a value in [%d, %d].", name, delta, netstatTestBytes, 4*netstatTestBytes)
		}
	}
}

// TestDarwinNetworkBytesManySockets verifies that the counters stay complete when
// the process owns more sockets than fit into the default receive buffer of the
// statistics socket.
func TestDarwinNetworkBytesManySockets(t *testing.T) {
	const (
		// Every connection has two sockets, 300 are well above the ~190 sources that fit
		// into the default receive buffer.
		conns   = 150
		perConn = 1000
	)
	rxBefore, txBefore := networkBytesOrSkip(t)

	for range conns {
		client, server := transferOverLoopback(t, perConn)
		defer client.Close()
		defer server.Close()
	}

	rxAfter, txAfter := networkBytesOrSkip(t)
	for name, delta := range map[string]int64{
		"received":    int64(rxAfter) - int64(rxBefore),
		"transmitted": int64(txAfter) - int64(txBefore),
	} {
		if delta < conns*perConn {
			t.Errorf("Bytes %s changed by %d, want at least %d.", name, delta, conns*perConn)
		}
	}
}

// TestDarwinNetworkBytesOtherProcesses verifies that the traffic of other
// processes is not attributed to the current one.
func TestDarwinNetworkBytesOtherProcesses(t *testing.T) {
	rxBefore, txBefore := networkBytesOrSkip(t)

	cmd := exec.Command(os.Args[0], "-test.run=^TestDarwinNetworkBytesHelperProcess$")
	cmd.Env = append(os.Environ(), netstatHelperEnv+"=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Closing stdin lets the helper exit.
		stdin.Close()
		_ = cmd.Wait()
	}()

	// The helper keeps its connection open after it printed this line.
	var output strings.Builder
	r := bufio.NewReader(stdout)
	for {
		line, err := r.ReadString('\n')
		output.WriteString(line)
		if err != nil {
			t.Fatalf("Helper process did not become ready: %v, output:\n%s", err, output.String())
		}
		if strings.TrimSpace(line) == "ready" {
			break
		}
	}

	rxAfter, txAfter := networkBytesOrSkip(t)

	// The helper moved netstatTestBytes in each direction, none of it is ours.
	for name, delta := range map[string]int64{
		"received":    int64(rxAfter) - int64(rxBefore),
		"transmitted": int64(txAfter) - int64(txBefore),
	} {
		if delta > netstatTestBytes/2 || delta < -netstatTestBytes/2 {
			t.Errorf("Bytes %s changed by %d while only another process transferred data.", name, delta)
		}
	}
}

// TestDarwinNetworkBytesHelperProcess is not a real test. It is run as a
// subprocess by TestDarwinNetworkBytesOtherProcesses.
func TestDarwinNetworkBytesHelperProcess(t *testing.T) {
	if os.Getenv(netstatHelperEnv) != "1" {
		t.Skip("Only runs as a helper process.")
	}

	client, server := transferOverLoopback(t, netstatTestBytes)
	defer client.Close()
	defer server.Close()

	_, _ = os.Stdout.WriteString("ready\n")
	// Keep the connection open until the parent is done.
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestForEachNstatMsg(t *testing.T) {
	encode := func(msgs ...any) []byte {
		var buf bytes.Buffer
		for _, m := range msgs {
			if err := binary.Write(&buf, binary.LittleEndian, m); err != nil {
				t.Fatal(err)
			}
		}
		return buf.Bytes()
	}
	counts := func(srcRef uint64) nstatMsgSrcCounts {
		return nstatMsgSrcCounts{
			Hdr:    nstatMsgHdr{Type: nstatMsgTypeSrcCounts, Length: 64},
			SrcRef: srcRef,
		}
	}
	success := nstatMsgHdr{Type: nstatMsgTypeSuccess, Length: nstatMsgHdrSize}

	for _, tc := range []struct {
		name      string
		data      []byte
		wantTypes []uint32
		wantErr   bool
	}{
		{
			name:      "single message",
			data:      encode(success),
			wantTypes: []uint32{nstatMsgTypeSuccess},
		},
		{
			name:      "several messages in one datagram",
			data:      encode(counts(1), counts(2), success),
			wantTypes: []uint32{nstatMsgTypeSrcCounts, nstatMsgTypeSrcCounts, nstatMsgTypeSuccess},
		},
		{
			name: "invalid length covers the rest of the datagram",
			data: encode(nstatMsgHdr{Type: nstatMsgTypeSrcCounts}, uint64(7)),
			// A single message with 8 bytes of payload.
			wantTypes: []uint32{nstatMsgTypeSrcCounts},
		},
		{
			name:    "truncated header",
			data:    encode(success)[:nstatMsgHdrSize-1],
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotTypes []uint32
			err := forEachNstatMsg(tc.data, func(hdr nstatMsgHdr, msg []byte) error {
				gotTypes = append(gotTypes, hdr.Type)
				if hdr.Type == nstatMsgTypeSrcCounts && len(msg) == 64 {
					var m nstatMsgSrcCounts
					if err := decodeNstatMsg(msg, &m); err != nil {
						return err
					}
					if m.SrcRef != uint64(len(gotTypes)) {
						t.Errorf("Got source reference %d, want %d.", m.SrcRef, len(gotTypes))
					}
				}
				return nil
			})
			if (err != nil) != tc.wantErr {
				t.Fatalf("Got error %v, want error: %t.", err, tc.wantErr)
			}
			if len(gotTypes) != len(tc.wantTypes) {
				t.Fatalf("Got message types %v, want %v.", gotTypes, tc.wantTypes)
			}
			for i := range gotTypes {
				if gotTypes[i] != tc.wantTypes[i] {
					t.Fatalf("Got message types %v, want %v.", gotTypes, tc.wantTypes)
				}
			}
		})
	}

	t.Run("callback error stops the iteration", func(t *testing.T) {
		stop := errors.New("stop")
		calls := 0
		err := forEachNstatMsg(encode(counts(1), counts(2)), func(nstatMsgHdr, []byte) error {
			calls++
			return stop
		})
		if !errors.Is(err, stop) || calls != 1 {
			t.Fatalf("Got error %v after %d calls, want %v after 1 call.", err, calls, stop)
		}
	})
}
