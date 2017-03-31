// Copyright 2016 The Prometheus Authors
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

// Copyright (c) 2013, The Prometheus Authors
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package promhttp

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

type doer interface {
	Do(*http.Request) (*http.Response, error)
}

// ClientMiddleware is an adapter to allow wrapping an http.Client or other
// Middleware funcs, allowing the user to construct layers of middleware around
// an http client request.
type ClientMiddleware func(req *http.Request) (*http.Response, error)

// Do implements the httpClient interface.
func (c ClientMiddleware) Do(r *http.Request) (*http.Response, error) {
	return c(r)
}

// Get implements the httpClient interface.
func (c ClientMiddleware) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Head implements the httpClient interface.
func (c ClientMiddleware) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post implements the httpClient interface.
func (c ClientMiddleware) Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// PostForm implements the httpClient interface.
func (c ClientMiddleware) PostForm(url string, data url.Values) (*http.Response, error) {
	return c.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}
