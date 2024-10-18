// Copyright 2024 The Prometheus Authors
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

package remotewrite

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	writev1 "github.com/prometheus/client_golang/api/remotewrite/genproto/v1"
	writev2 "github.com/prometheus/client_golang/api/remotewrite/genproto/v2"
)

const (
	VersionHeader        = "X-Prometheus-Remote-Write-Version"
	Version1HeaderValue  = "0.1.0"
	Version20HeaderValue = "2.0.0"
	appProtoContentType  = "application/x-protobuf"
)

// Compression represents the encoding. Currently remote storage supports only
// one, but we experiment with more, thus leaving the compression scaffolding
// for now.
type Compression string

const (
	// SnappyBlockCompression represents https://github.com/google/snappy/blob/2c94e11145f0b7b184b831577c93e5a41c4c0346/format_description.txt
	SnappyBlockCompression Compression = "snappy"
)

// ProtoMsg represents the known protobuf message for the remote write 1.0 and 2.0 specs.
type ProtoMsg string

const (
	// ProtoMsgV1 represents the `prometheus.WriteRequest` protobuf
	// message introduced in the https://prometheus.io/docs/specs/remote_write_spec/,
	// which will eventually be deprecated.
	ProtoMsgV1 ProtoMsg = "prometheus.WriteRequest"
	// ProtoMsgV2 represents the `io.prometheus.write.v2.Request` protobuf
	// message introduced in https://prometheus.io/docs/specs/remote_write_spec_2_0/
	ProtoMsgV2 ProtoMsg = "io.prometheus.write.v2.Request"
)

// Validate returns error if the given reference for the protobuf message is not supported.
func (s ProtoMsg) Validate() error {
	switch s {
	case ProtoMsgV1, ProtoMsgV2:
		return nil
	default:
		return fmt.Errorf("unknown remote write protobuf message %v, supported: %v", s, protoMsgs{ProtoMsgV1, ProtoMsgV2}.String())
	}
}

type protoMsgs []ProtoMsg

func (m protoMsgs) Strings() []string {
	ret := make([]string, 0, len(m))
	for _, typ := range m {
		ret = append(ret, string(typ))
	}
	return ret
}

func (m protoMsgs) String() string {
	return strings.Join(m.Strings(), ", ")
}

var contentTypeHeaders = map[ProtoMsg]string{
	ProtoMsgV1: appProtoContentType, // Also application/x-protobuf;proto=prometheus.WriteRequest but simplified for compatibility with 1.x spec.
	ProtoMsgV2: appProtoContentType + ";proto=io.prometheus.write.v2.Request",
}

// ContentTypeHeader returns content type header value for the given proto message
// or empty string for unknown proto message.
func ContentTypeHeader(m ProtoMsg) string {
	return contentTypeHeaders[m]
}

const (
	WrittenSamplesHeader    = "X-Prometheus-Remote-Write-Samples-Written"
	WrittenHistogramsHeader = "X-Prometheus-Remote-Write-Histograms-Written"
	WrittenExemplarsHeader  = "X-Prometheus-Remote-Write-Exemplars-Written"
)

// WriteResponseStats represents the response write statistics.
type WriteResponseStats struct {
	// Samples represents X-Prometheus-Remote-Write-Written-Samples
	Samples int
	// Histograms represents X-Prometheus-Remote-Write-Written-Histograms
	Histograms int
	// Exemplars represents X-Prometheus-Remote-Write-Written-Exemplars
	Exemplars int

	// Confirmed means we can trust those statistics from the point of view
	// of the PRW 2.0 spec. When parsed from headers, it means we got at least one
	// response header from the Receiver to confirm those numbers, meaning it must
	// be at least 2.0 Receiver. See ParseWriteResponseStats for details.
	Confirmed bool
}

// NoDataWritten returns true if statistics indicate no data was written.
func (s WriteResponseStats) NoDataWritten() bool {
	return (s.Samples + s.Histograms + s.Exemplars) == 0
}

// AllSamples returns both float and histogram sample numbers.
func (s WriteResponseStats) AllSamples() int {
	return s.Samples + s.Histograms
}

// Add returns the sum of this WriteResponseStats plus the given WriteResponseStats.
func (s WriteResponseStats) Add(rs WriteResponseStats) WriteResponseStats {
	s.Confirmed = rs.Confirmed
	s.Samples += rs.Samples
	s.Histograms += rs.Histograms
	s.Exemplars += rs.Exemplars
	return s
}

// AddV1 returns the sum of this WriteResponseStats plus the statistics from v1 request.
func (s WriteResponseStats) AddV1(req *writev1.WriteRequest) WriteResponseStats {
	s.Confirmed = true
	for _, ts := range req.Timeseries {
		s.Samples += len(ts.Samples)
		s.Histograms += len(ts.Histograms)
		s.Exemplars += len(ts.Exemplars)
	}
	return s
}

// AddV2 returns the sum of this WriteResponseStats plus the statistics from v2 request.
func (s WriteResponseStats) AddV2(req *writev2.Request) WriteResponseStats {
	s.Confirmed = true
	for _, ts := range req.Timeseries {
		s.Samples += len(ts.Samples)
		s.Histograms += len(ts.Histograms)
		s.Exemplars += len(ts.Exemplars)
	}
	return s
}

// SetHeaders sets response headers in a given response writer.
// Make sure to use it before http.ResponseWriter.WriteHeader and .Write.
func (s WriteResponseStats) SetHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set(WrittenSamplesHeader, strconv.Itoa(s.Samples))
	h.Set(WrittenHistogramsHeader, strconv.Itoa(s.Histograms))
	h.Set(WrittenExemplarsHeader, strconv.Itoa(s.Exemplars))
}

// parseWriteResponseStats returns WriteResponseStats parsed from the response headers.
//
// As per 2.0 spec, missing header means 0. However, abrupt HTTP errors, 1.0 Receivers
// or buggy 2.0 Receivers might result in no response headers specified and that
// might NOT necessarily mean nothing was written. To represent that we set
// s.Confirmed = true only when see at least on response header.
//
// Error is returned when any of the header fails to parse as int64.
func parseWriteResponseStats(r *http.Response) (s WriteResponseStats, err error) {
	var (
		errs []error
		h    = r.Header
	)
	if v := h.Get(WrittenSamplesHeader); v != "" { // Empty means zero.
		s.Confirmed = true
		if s.Samples, err = strconv.Atoi(v); err != nil {
			s.Samples = 0
			errs = append(errs, err)
		}
	}
	if v := h.Get(WrittenHistogramsHeader); v != "" { // Empty means zero.
		s.Confirmed = true
		if s.Histograms, err = strconv.Atoi(v); err != nil {
			s.Histograms = 0
			errs = append(errs, err)
		}
	}
	if v := h.Get(WrittenExemplarsHeader); v != "" { // Empty means zero.
		s.Confirmed = true
		if s.Exemplars, err = strconv.Atoi(v); err != nil {
			s.Exemplars = 0
			errs = append(errs, err)
		}
	}
	return s, errors.Join(errs...)
}
