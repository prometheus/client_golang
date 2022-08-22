// Copyright 2014 The Prometheus Authors
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

package errcapture

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

type doFunc func() error

// Do runs function and on error return error by argument including the given error (usually
// from caller function).
func Do(err *error, doer doFunc, format string, a ...interface{}) {
	derr := doer()
	if err == nil || derr == nil {
		return
	}

	// For os closers, it's a common case to double close.
	// From reliability purpose this is not a problem it may only indicate surprising execution path.
	if errors.Is(derr, os.ErrClosed) {
		return
	}

	errs := prometheus.MultiError{}
	errs.Append(*err)
	errs.Append(fmt.Errorf(format+": %w", append(a, derr)...))
	*err = errs
}

// ExhaustClose closes the io.ReadCloser with error capture but exhausts the reader before.
func ExhaustClose(err *error, r io.ReadCloser, format string, a ...interface{}) {
	_, copyErr := io.Copy(ioutil.Discard, r)

	Do(err, r.Close, format, a...)
	if copyErr == nil {
		return
	}

	errs := prometheus.MultiError{}
	errs.Append(copyErr)
	errs.Append(*err)
	*err = errs
}
