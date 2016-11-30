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

import "net/http"

type Client struct {
	// Do we want to allow users to modify the underlying client after creating the wrapping client?
	// c defaults to the http DefaultClient. We use a pointer to support
	// the user relying on modifying this behavior globally.
	c *http.Client

	middleware Middleware
}

type ConfigFunc func(*Client) error

type Middleware func(req *http.Request) (*http.Response, error)

type middlewareFunc func(req *http.Request, next Middleware) (*http.Response, error)

func (c *Client) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	return c.middleware(req)
}

func (c *Client) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.middleware(req)
}

func (c *Client) do(r *http.Request, _ middlewareFunc) (*http.Response, error) {
	return c.c.Do(r)
}

func SetMiddleware(middlewares ...middlewareFunc) ConfigFunc {
	return func(c *Client) error {
		c.middleware = build(c, middlewares...)
		return nil
	}
}

func build(c *Client, middlewares ...middlewareFunc) Middleware {
	if len(middlewares) == 0 {
		return endMiddleware(c)
	}

	next := build(c, middlewares[1:]...)

	return wrap(middlewares[0], next)
}

func wrap(fn middlewareFunc, next Middleware) Middleware {
	return Middleware(func(r *http.Request) (*http.Response, error) {
		return fn(r, next)
	})
}

func endMiddleware(c *Client) Middleware {
	return Middleware(func(r *http.Request) (*http.Response, error) {
		return c.do(r, nil)
	})
}

// SetClient sets the http client on Client. It accepts a value as opposed to a
// pointer to discourage untraced modification of a custom http client after it
// has been set.
func SetClient(client http.Client) ConfigFunc {
	return func(c *Client) error {
		c.c = &client
		return nil
	}
}

func NewClient(fns ...ConfigFunc) (*Client, error) {
	c := &Client{
		c: http.DefaultClient,
	}
	c.middleware = build(c)

	for _, fn := range fns {
		if err := fn(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}
