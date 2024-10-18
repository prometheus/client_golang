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
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/klauspost/compress/snappy"
	writev1 "github.com/prometheus/client_golang/api/remotewrite/genproto/v1"
	writev2 "github.com/prometheus/client_golang/api/remotewrite/genproto/v2"
)

type Storage interface {
	Store(ctx context.Context, proto ProtoMsg, serializedRequest []byte) (_ WriteResponseStats, code int, _ error)
}

type Handler struct {
	logger *slog.Logger
	store  Storage
}

type DecodedStorage interface {
	StoreV1(ctx context.Context, reqV1 *writev1.WriteRequest) (_ *WriteResponseStats, code *int, _ error)
	StoreV2(ctx context.Context, reqV2 *writev2.Request) (_ *WriteResponseStats, code *int, _ error)
}

func NewDecodingStore(store DecodedStorage) Storage {
	return &decodingStore{store: store}
}

type decodingStore struct {
	store DecodedStorage
}

func (s *decodingStore) Store(ctx context.Context, proto ProtoMsg, serializedRequest []byte) (stats WriteResponseStats, code int, _ error) {
	var (
		maybeStats *WriteResponseStats
		maybeCode  *int
		err        error
	)

	switch proto {
	case ProtoMsgV1:
		var req writev1.WriteRequest
		if err := req.UnmarshalVT(serializedRequest); err != nil {
			return stats, http.StatusBadRequest, fmt.Errorf("decoding v1 remote write request: %w", err)
		}

		maybeStats, maybeCode, err = s.store.StoreV1(ctx, &req)
		if maybeStats != nil {
			stats = *maybeStats
		} else {
			stats = stats.AddV1(&req)
		}

	case ProtoMsgV2:
		var req writev2.Request
		if err := req.UnmarshalVT(serializedRequest); err != nil {
			return stats, http.StatusBadRequest, fmt.Errorf("decoding v2 remote write request: %w", err)
		}
		maybeStats, maybeCode, err = s.store.StoreV2(ctx, &req)
		if maybeStats != nil {
			stats = *maybeStats
		} else {
			stats = stats.AddV2(&req)
		}
	default:
		return stats, http.StatusUnsupportedMediaType, fmt.Errorf("unsupported proto format %v", string(proto))
	}

	if err != nil {
		if maybeCode == nil {
			return stats, http.StatusInternalServerError, err
		}
		return stats, *maybeCode, err
	}
	if maybeCode == nil {
		return stats, http.StatusOK, nil
	}
	return stats, *maybeCode, nil
}

// TODO(bwplotka): Add variadic options if needed.
func NewHandler(logger *slog.Logger, store Storage) *Handler {
	return &Handler{logger: logger, store: store}
}

func parseProtoMsg(contentType string) (ProtoMsg, error) {
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
			ret := ProtoMsg(pair[1])
			if err := ret.Validate(); err != nil {
				return "", fmt.Errorf("got %v content type; %w", contentType, err)
			}
			return ret, nil
		}
	}
	// No "proto=" parameter, assuming v1.
	return ProtoMsgV1, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
