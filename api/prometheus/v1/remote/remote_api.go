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

// Package remote implements bindings for Prometheus Remote APIs.
package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/snappy"
	"google.golang.org/protobuf/proto"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/internal/github.com/efficientgo/core/backoff"
)

// API is a client for Prometheus Remote Protocols.
// NOTE(bwplotka): Only https://prometheus.io/docs/specs/remote_write_spec_2_0/ is currently implemented,
// read protocols to be implemented if there will be a demand.
type API struct {
	client api.Client
	opts   apiOpts

	reqBuf, comprBuf []byte
}

// APIOption represents a remote API option.
type APIOption func(o *apiOpts) error

// TODO(bwplotka): Add "too old sample" handling one day.
type apiOpts struct {
	logger           *slog.Logger
	backoff          backoff.Config
	compression      Compression
	retryOnRateLimit bool
}

var defaultAPIOpts = &apiOpts{
	backoff: backoff.Config{
		Min:        1 * time.Second,
		Max:        10 * time.Second,
		MaxRetries: 10,
	},
	// Hardcoded for now.
	retryOnRateLimit: true,
	compression:      SnappyBlockCompression,
}

// WithAPILogger returns APIOption that allows providing slog logger.
// By default, nothing is logged.
func WithAPILogger(logger *slog.Logger) APIOption {
	return func(o *apiOpts) error {
		o.logger = logger
		return nil
	}
}

type nopSlogHandler struct{}

func (n nopSlogHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (n nopSlogHandler) Handle(context.Context, slog.Record) error { return nil }
func (n nopSlogHandler) WithAttrs([]slog.Attr) slog.Handler        { return n }
func (n nopSlogHandler) WithGroup(string) slog.Handler             { return n }

// NewAPI returns a new API for the clients of Remote Write Protocol.
//
// It is not safe to use the returned API from multiple goroutines, create a
// separate *API for each goroutine.
func NewAPI(c api.Client, opts ...APIOption) (*API, error) {
	o := *defaultAPIOpts
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	if o.logger == nil {
		o.logger = slog.New(nopSlogHandler{})
	}

	return &API{
		client: c,
		opts:   o,
	}, nil
}

type retryableError struct {
	error
	retryAfter time.Duration
}

func (r retryableError) RetryAfter() time.Duration {
	return r.retryAfter
}

type vtProtoEnabled interface {
	SizeVT() int
	MarshalToSizedBufferVT(dAtA []byte) (int, error)
}

// Write writes given, non-empty, protobuf message to a remote storage.
// The https://github.com/planetscale/vtprotobuf methods will be used if your msg
// supports those (e.g. SizeVT() and MarshalToSizedBufferVT(...)), for efficiency.
func (r *API) Write(ctx context.Context, msg proto.Message) (_ WriteResponseStats, err error) {
	// Detect content-type.
	cType := WriteProtoFullName(proto.MessageName(msg))
	if err := cType.Validate(); err != nil {
		return WriteResponseStats{}, err
	}

	// Encode the payload.
	if emsg, ok := msg.(vtProtoEnabled); ok {
		// Use optimized vtprotobuf if supported.
		size := emsg.SizeVT()
		if len(r.reqBuf) < size {
			r.reqBuf = make([]byte, size)
		}
		if _, err := emsg.MarshalToSizedBufferVT(r.reqBuf[:size]); err != nil {
			return WriteResponseStats{}, fmt.Errorf("encoding request %w", err)
		}
	} else {
		// Generic proto.
		r.reqBuf = r.reqBuf[:0]
		r.reqBuf, err = (proto.MarshalOptions{}).MarshalAppend(r.reqBuf, msg)
		if err != nil {
			return WriteResponseStats{}, fmt.Errorf("encoding request %w", err)
		}
	}

	payload, err := compressPayload(&r.comprBuf, r.opts.compression, r.reqBuf)
	if err != nil {
		return WriteResponseStats{}, fmt.Errorf("compressing %w", err)
	}

	// Since we retry writes we need to track the total amount of accepted data
	// across the various attempts.
	accumulatedStats := WriteResponseStats{}

	b := backoff.New(ctx, r.opts.backoff)
	for {
		rs, err := r.attemptWrite(ctx, r.opts.compression, cType, payload, b.NumRetries())
		accumulatedStats = accumulatedStats.Add(rs)
		if err == nil {
			// Check the case mentioned in PRW 2.0.
			// https://prometheus.io/docs/specs/remote_write_spec_2_0/#required-written-response-headers.
			if cType == WriteProtoFullNameV2 && !accumulatedStats.Confirmed && accumulatedStats.NoDataWritten() {
				// TODO(bwplotka): Allow users to disable this check or provide their stats for us to know if it's empty.
				return accumulatedStats, fmt.Errorf("sent v2 request; "+
					"got 2xx, but PRW 2.0 response header statistics indicate %v samples, %v histograms "+
					"and %v exemplars were accepted; assumining failure e.g. the target only supports "+
					"PRW 1.0 prometheus.WriteRequest, but does not check the Content-Type header correctly",
					accumulatedStats.Samples, accumulatedStats.Histograms, accumulatedStats.Exemplars,
				)
			}
			// Success!
			// TODO(bwplotka): Debug log with retry summary?
			return accumulatedStats, nil
		}

		var retryableErr retryableError
		if !errors.As(err, &retryableErr) {
			// TODO(bwplotka): More context in the error e.g. about retries.
			return accumulatedStats, err
		}

		if !b.Ongoing() {
			// TODO(bwplotka): More context in the error e.g. about retries.
			return accumulatedStats, err
		}

		backoffDelay := b.NextDelay() + retryableErr.RetryAfter()
		r.opts.logger.Error("failed to send remote write request; retrying after backoff", "err", err, "backoff", backoffDelay)
		select {
		case <-ctx.Done():
			// TODO(bwplotka): More context in the error e.g. about retries.
			return WriteResponseStats{}, ctx.Err()
		case <-time.After(backoffDelay):
			// Retry.
		}
	}
}

func compressPayload(tmpbuf *[]byte, enc Compression, inp []byte) (compressed []byte, _ error) {
	switch enc {
	case SnappyBlockCompression:
		compressed = snappy.Encode(*tmpbuf, inp)
		if n := snappy.MaxEncodedLen(len(inp)); n > len(*tmpbuf) {
			// grow the buffer for the next time.
			*tmpbuf = make([]byte, n)
		}
		return compressed, nil
	default:
		return compressed, fmt.Errorf("unknown compression scheme [%v]", enc)
	}
}

func (r *API) attemptWrite(ctx context.Context, compr Compression, proto WriteProtoFullName, payload []byte, attempt int) (WriteResponseStats, error) {
	u := r.client.URL("api/v1/write", nil)
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		// Errors from NewRequest are from unparsable URLs, so are not
		// recoverable.
		return WriteResponseStats{}, err
	}

	req.Header.Add("Content-Encoding", string(compr))
	req.Header.Set("Content-Type", contentTypeHeader(proto))
	if proto == WriteProtoFullNameV1 {
		// Compatibility mode for 1.0.
		req.Header.Set(versionHeader, version1HeaderValue)
	} else {
		req.Header.Set(versionHeader, version20HeaderValue)
	}

	if attempt > 0 {
		req.Header.Set("Retry-Attempt", strconv.Itoa(attempt))
	}

	resp, body, err := r.client.Do(ctx, req)
	if err != nil {
		// Errors from Client.Do are likely network errors, so recoverable.
		return WriteResponseStats{}, retryableError{err, 0}
	}

	rs, err := parseWriteResponseStats(resp)
	if err != nil {
		r.opts.logger.Warn("parsing rw write statistics failed; partial or no stats", "err", err)
	}

	if resp.StatusCode/100 == 2 {
		return rs, nil
	}

	err = fmt.Errorf("server returned HTTP status %s: %s", resp.Status, body)
	if resp.StatusCode/100 == 5 ||
		(r.opts.retryOnRateLimit && resp.StatusCode == http.StatusTooManyRequests) {
		return rs, retryableError{err, retryAfterDuration(resp.Header.Get("Retry-After"))}
	}
	return rs, err
}

// retryAfterDuration returns the duration for the Retry-After header. In case of any errors, it
// returns 0 as if the header was never supplied.
func retryAfterDuration(t string) time.Duration {
	parsedDuration, err := time.Parse(http.TimeFormat, t)
	if err == nil {
		return time.Until(parsedDuration)
	}
	// The duration can be in seconds.
	d, err := strconv.Atoi(t)
	if err != nil {
		return 0
	}
	return time.Duration(d) * time.Second
}

// writeStorage represents the storage for RemoteWriteHandler.
// This interface is intentionally private due its experimental state.
type writeStorage interface {
	Store(ctx context.Context, proto WriteProtoFullName, serializedRequest []byte) (_ WriteResponseStats, code int, _ error)
}

type handler struct {
	logger *slog.Logger
	store  writeStorage
}

// NewRemoteWriteHandler returns HTTP handler that receives Remote Write 2.0
// protocol https://prometheus.io/docs/specs/remote_write_spec_2_0/.
func NewRemoteWriteHandler(logger *slog.Logger, store writeStorage) http.Handler {
	return &handler{logger: logger, store: store}
}

func parseProtoMsg(contentType string) (WriteProtoFullName, error) {
	contentType = strings.TrimSpace(contentType)

	parts := strings.Split(contentType, ";")
	if parts[0] != appProtoContentType {
		return "", fmt.Errorf("expected %v as the first (media) part, got %v content-type", appProtoContentType, contentType)
	}
	// Parse potential https://www.rfc-editor.org/rfc/rfc9110#parameter
	for _, p := range parts[1:] {
		pair := strings.Split(p, "=")
		if len(pair) != 2 {
			return "", fmt.Errorf("as per https://www.rfc-editor.org/rfc/rfc9110#parameter expected parameters to be key-values, got %v in %v content-type", p, contentType)
		}
		if pair[0] == "proto" {
			ret := WriteProtoFullName(pair[1])
			if err := ret.Validate(); err != nil {
				return "", fmt.Errorf("got %v content type; %w", contentType, err)
			}
			return ret, nil
		}
	}
	// No "proto=" parameter, assuming v1.
	return WriteProtoFullNameV1, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		// Don't break yolo 1.0 clients if not needed.
		// We could give http.StatusUnsupportedMediaType, but let's assume 1.0 message by default.
		contentType = appProtoContentType
	}

	msgType, err := parseProtoMsg(contentType)
	if err != nil {
		h.logger.Error("Error decoding remote write request", "err", err)
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}

	enc := r.Header.Get("Content-Encoding")
	if enc == "" {
		// Don't break yolo 1.0 clients if not needed. This is similar to what we did
		// before 2.0: https://github.com/prometheus/prometheus/blob/d78253319daa62c8f28ed47e40bafcad2dd8b586/storage/remote/write_handler.go#L62
		// We could give http.StatusUnsupportedMediaType, but let's assume snappy by default.
	} else if enc != string(SnappyBlockCompression) {
		err := fmt.Errorf("%v encoding (compression) is not accepted by this server; only %v is acceptable", enc, SnappyBlockCompression)
		h.logger.Error("Error decoding remote write request", "err", err)
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
	}

	// Read the request body.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Error decoding remote write request", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	decompressed, err := snappy.Decode(nil, body)
	if err != nil {
		// TODO(bwplotka): Add more context to responded error?
		h.logger.Error("Error decompressing remote write request", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stats, code, storeErr := h.store.Store(r.Context(), msgType, decompressed)

	// Set required X-Prometheus-Remote-Write-Written-* response headers, in all cases.
	stats.SetHeaders(w)

	if storeErr != nil {
		if code == 0 {
			code = http.StatusInternalServerError
		}
		if code/5 == 100 { // 5xx
			h.logger.Error("Error while remote writing the v2 request", "err", storeErr.Error())
		}
		http.Error(w, storeErr.Error(), code)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
