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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/efficientgo/core/backoff"
	"github.com/klauspost/compress/snappy"
	writev1 "github.com/prometheus/client_golang/api/remotewrite/genproto/v1"
	writev2 "github.com/prometheus/client_golang/api/remotewrite/genproto/v2"
)

const (
	defaultBackoff = 0
	maxErrMsgLen   = 1024
)

type Client struct {
	logger *slog.Logger
	url    string
	client *http.Client

	userAgent        string
	retryOnRateLimit bool

	compr    Compression
	comprBuf []byte

	b *backoff.Backoff
}

type EncodingClient struct {
	client *Client

	buf []byte
}

func NewEncodingClient(client *Client) *EncodingClient {
	return &EncodingClient{client: client}
}

func (c *EncodingClient) WriteV1(ctx context.Context, req *writev1.WriteRequest, opts *ClientWriteOpts) (WriteResponseStats, error) {
	size := req.SizeVT()
	if len(c.buf) < size {
		c.buf = make([]byte, size)
	}
	if _, err := req.MarshalToSizedBufferVT(c.buf[:size]); err != nil {
		return WriteResponseStats{}, fmt.Errorf("encoding v1 request %w", err)
	}
	return c.client.Write(ctx, ProtoMsgV1, c.buf[:size], opts)
}

func (c *EncodingClient) WriteV2(ctx context.Context, req *writev2.Request, opts *ClientWriteOpts) (WriteResponseStats, error) {
	size := req.SizeVT()
	if len(c.buf) < size {
		c.buf = make([]byte, size)
	}
	if _, err := req.MarshalToSizedBufferVT(c.buf[:size]); err != nil {
		return WriteResponseStats{}, fmt.Errorf("encoding v2 request %w", err)
	}
	stats, err := c.client.Write(ctx, ProtoMsgV2, c.buf[:size], opts)
	if err != nil {
		return stats, err
	}

	// Check the case mentioned in PRW 2.0.
	// https://prometheus.io/docs/specs/remote_write_spec_2_0/#required-written-response-headers.
	if !stats.Confirmed && stats.NoDataWritten() {
		cStats := WriteResponseStats{}.AddV2(req)
		if !cStats.NoDataWritten() {
			return stats, fmt.Errorf("sent v2 request with %v samples %v histograms %v exemplars; "+
				"got 2xx, but PRW 2.0 response header statistics indicate %v samples, %v histograms "+
				"and %v exemplars were accepted; assumining failure e.g. the target only supports "+
				"PRW 1.0 prometheus.WriteRequest, but does not check the Content-Type header correctly",
				cStats.Samples, cStats.Histograms, cStats.Exemplars,
				stats.Samples, stats.Histograms, stats.Exemplars,
			)
		}
	}
	return stats, nil
}

// NewClient returns client.
// TODO(bwplotka): Add variadic options.
func NewClient(logger *slog.Logger, url string, hc *http.Client, compr Compression, ua string, retryOnRateLimit bool) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 1 * time.Minute}
	}
	return &Client{
		logger:           logger,
		url:              url,
		client:           hc,
		compr:            compr,
		userAgent:        ua,
		retryOnRateLimit: retryOnRateLimit,
	}
}

type RetryableError struct {
	error
	retryAfter time.Duration
}

func (r RetryableError) RetryAfter() time.Duration {
	return r.retryAfter
}

type ClientWriteOpts struct {
	Backoff backoff.Config
}

var defaultOpts = &ClientWriteOpts{
	Backoff: backoff.Config{
		Min:        1 * time.Second,
		Max:        10 * time.Second,
		MaxRetries: 10,
	},
}

// TODO(bwplotka): Support variadic options allowing too old sample handling, tracing, metrics
func (c *Client) Write(ctx context.Context, proto ProtoMsg, serializedRequest []byte, opts *ClientWriteOpts) (WriteResponseStats, error) {
	o := *defaultOpts
	if opts != nil {
		o = *opts
	}
	payload, err := compressPayload(&c.comprBuf, c.compr, serializedRequest)
	if err != nil {
		return WriteResponseStats{}, fmt.Errorf("compressing %w", err)
	}

	// Since we retry writes we need to track the total amount of accepted data
	// across the various attempts.
	accumulatedStats := WriteResponseStats{}

	b := backoff.New(ctx, o.Backoff)
	for {
		rs, err := c.write(ctx, proto, payload, b.NumRetries())
		accumulatedStats = accumulatedStats.Add(rs)
		if err == nil {
			// Success!
			// TODO(bwplotka): Debug log with retry summary?
			return accumulatedStats, nil
		}

		var retryableErr RetryableError
		if !errors.As(err, &retryableErr) {
			// TODO(bwplotka): More context in the error e.g. about retries.
			return accumulatedStats, err
		}

		if !b.Ongoing() {
			// TODO(bwplotka): More context in the error e.g. about retries.
			return accumulatedStats, err
		}

		backoffDelay := b.NextDelay() + retryableErr.RetryAfter()
		c.logger.Error("failed to send remote write request; retrying after backoff", "err", err, "backoff", backoffDelay)
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
		return compressed, fmt.Errorf("Unknown compression scheme [%v]", enc)
	}
}

func (c *Client) write(ctx context.Context, proto ProtoMsg, payload []byte, attempt int) (WriteResponseStats, error) {
	httpReq, err := http.NewRequest(http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		// Errors from NewRequest are from unparsable URLs, so are not
		// recoverable.
		return WriteResponseStats{}, err
	}

	httpReq.Header.Add("Content-Encoding", string(c.compr))
	httpReq.Header.Set("Content-Type", ContentTypeHeader(proto))
	httpReq.Header.Set("User-Agent", c.userAgent)
	if proto == ProtoMsgV1 {
		// Compatibility mode for 1.0.
		httpReq.Header.Set(VersionHeader, Version1HeaderValue)
	} else {
		httpReq.Header.Set(VersionHeader, Version20HeaderValue)
	}

	if attempt > 0 {
		httpReq.Header.Set("Retry-Attempt", strconv.Itoa(attempt))
	}

	httpResp, err := c.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		// Errors from Client.Do are likely network errors, so recoverable.
		return WriteResponseStats{}, RetryableError{err, defaultBackoff}
	}
	defer func() {
		_, _ = io.Copy(io.Discard, httpResp.Body)
		_ = httpResp.Body.Close()
	}()

	rs, err := parseWriteResponseStats(httpResp)
	if err != nil {
		c.logger.Warn("parsing rw write statistics failed; partial or no stats", "err", err)
	}

	if httpResp.StatusCode/100 == 2 {
		return rs, nil
	}

	body, err := io.ReadAll(io.LimitReader(httpResp.Body, maxErrMsgLen))
	err = fmt.Errorf("server returned HTTP status %s: %s", httpResp.Status, body)

	if httpResp.StatusCode/100 == 5 ||
		(c.retryOnRateLimit && httpResp.StatusCode == http.StatusTooManyRequests) {
		return rs, RetryableError{err, retryAfterDuration(httpResp.Header.Get("Retry-After"))}
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
