// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package exp

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
)

const (
	unknownStatusCode = "unknown"
	statusFieldName   = "status"
)

type status string

func (s status) unknown() bool {
	return len(s) == 0
}

func (s status) String() string {
	if s.unknown() {
		return unknownStatusCode
	}

	return string(s)
}

func computeApproximateRequestSize(r http.Request) (s int) {
	s += len(r.Method)
	if r.URL != nil {
		s += len(r.URL.String())
	}
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}

	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}

	return
}

// ResponseWriterDelegator is a means of wrapping http.ResponseWriter to divine
// the response code from a given answer, especially in systems where the
// response is treated as a blackbox.
type ResponseWriterDelegator struct {
	http.ResponseWriter
	status       status
	BytesWritten int
}

func (r ResponseWriterDelegator) String() string {
	return fmt.Sprintf("ResponseWriterDelegator decorating %s with status %s and %d bytes written.", r.ResponseWriter, r.status, r.BytesWritten)
}

func (r *ResponseWriterDelegator) WriteHeader(code int) {
	r.status = status(strconv.Itoa(code))

	r.ResponseWriter.WriteHeader(code)
}

func (r *ResponseWriterDelegator) Status() string {
	if r.status.unknown() {
		delegate := reflect.ValueOf(r.ResponseWriter).Elem()
		statusField := delegate.FieldByName(statusFieldName)
		if statusField.IsValid() {
			r.status = status(strconv.Itoa(int(statusField.Int())))
		}
	}

	return r.status.String()
}

func (r *ResponseWriterDelegator) Write(b []byte) (n int, err error) {
	n, err = r.ResponseWriter.Write(b)
	r.BytesWritten += n
	return
}

func NewResponseWriterDelegator(delegate http.ResponseWriter) *ResponseWriterDelegator {
	return &ResponseWriterDelegator{
		ResponseWriter: delegate,
	}
}
