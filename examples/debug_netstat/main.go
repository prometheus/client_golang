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

// A debugging script using client_golang process collector code to reproduce and inspect
// process_network_receive_bytes_total and process_network_transmit_bytes_total metrics,
// specifically helping debug why network bytes may evaluate to 0 on macOS (darwin).
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

func main() {
	pid := os.Getpid()
	fmt.Printf("=== Process Collector Network Bytes Debugger ===\n")
	fmt.Printf("OS: %s, Architecture: %s, PID: %d\n\n", runtime.GOOS, runtime.GOARCH, pid)

	// 1. Initial collection using client_golang ProcessCollector
	reg := prometheus.NewRegistry()
	collector := collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
	reg.MustRegister(collector)

	fmt.Println("--- 1. Initial Gathering from client_golang ProcessCollector ---")
	gatherAndPrintNetworkMetrics(reg)

	// 2. Generate active HTTP and TCP network traffic
	fmt.Println("\n--- 2. Generating Active Network Traffic ---")
	bytesSent, bytesRecv := generateNetworkTraffic()
	fmt.Printf("Generated traffic: ~%d bytes sent, ~%d bytes received\n", bytesSent, bytesRecv)

	// 3. Gathering from client_golang ProcessCollector after traffic
	fmt.Println("\n--- 3. Post-Traffic Gathering from client_golang ProcessCollector ---")
	gatherAndPrintNetworkMetrics(reg)

	// 4. Low-level platform diagnostics
	runPlatformDiagnostics(pid)
}

func gatherAndPrintNetworkMetrics(reg *prometheus.Registry) {
	mfs, err := reg.Gather()
	if err != nil {
		log.Fatalf("Gather failed: %v", err)
	}

	foundMetrics := false
	for _, mf := range mfs {
		name := mf.GetName()
		if name == "process_network_receive_bytes_total" || name == "process_network_transmit_bytes_total" {
			foundMetrics = true
			for _, m := range mf.GetMetric() {
				if m.Counter != nil {
					fmt.Printf("  %s: %.0f bytes\n", name, m.Counter.GetValue())
				}
			}
		}
	}
	if !foundMetrics {
		fmt.Println("  (network metrics not exposed on this platform)")
	}
}

func generateNetworkTraffic() (int, int) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("DEBUG_NETWORK_BYTES_RESPONSE"))
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Printf("Failed to listen on local interface: %v", err)
		return 0, 0
	}
	defer listener.Close()

	server := &http.Server{Handler: handler}
	go server.Serve(listener)
	defer server.Close()

	var sent, recv int
	for i := 0; i < 10; i++ {
		resp, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			log.Printf("HTTP request error: %v", err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		recv += len(body)
		sent += 50 // approximate request size
	}
	time.Sleep(100 * time.Millisecond)
	return sent, recv
}
