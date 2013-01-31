/*
Copyright (c) 2013, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style license that can be found in
the LICENSE file.
*/

package contributor

import (
	"net/http"
	"strconv"
)

const (
	unknownStatusCode = "unknown"
)

// ResponseWriterDelegator is a means of wrapping http.ResponseWriter to divine
// the response code from a given answer, especially in systems where the
// response is treated as a blackbox.
type ResponseWriterDelegator struct {
	http.ResponseWriter
	Status *string
}

func (r ResponseWriterDelegator) WriteHeader(code int) {
	*r.Status = strconv.Itoa(code)

	r.ResponseWriter.WriteHeader(code)
}

func NewResponseWriterDelegator(delegate http.ResponseWriter) ResponseWriterDelegator {
	defaultStatusCode := unknownStatusCode

	return ResponseWriterDelegator{delegate, &defaultStatusCode}
}
