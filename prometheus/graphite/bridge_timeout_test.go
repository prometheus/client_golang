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

package graphite_test

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/graphite"
)

func TestPushWriteTimeout(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}

	registry := prometheus.NewRegistry()
	metric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "large_metric",
		ConstLabels: prometheus.Labels{
			"value": strings.Repeat("x", 8*1024*1024),
		},
	})
	registry.MustRegister(metric)
	bridge, err := graphite.NewBridge(&graphite.Config{
		URL: listener.Addr().String(), Gatherer: registry, Timeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		result <- bridge.Push()
		close(done)
	}()
	var peer *net.TCPConn
	defer func() {
		if peer != nil {
			peer.Close()
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Push did not return after closing the peer")
		}
	}()
	peer, err = listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	if err := peer.SetReadBuffer(1024); err != nil {
		t.Fatal(err)
	}
	// The peer accepts the connection but never reads, so the push fills its send buffer.
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case err := <-result:
		var timeoutErr net.Error
		if !errors.As(err, &timeoutErr) || !timeoutErr.Timeout() {
			t.Fatalf("Push error = %v, want a write timeout", err)
		}
	case <-timer.C:
		t.Fatal("Push remained blocked after the configured timeout")
	}
}
