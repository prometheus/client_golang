// Copyright 2013 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

package httputil

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/internal/github.com/golang/gddo/httputil/header"
)

// NegotiateContentEncoding returns the best offered content encoding for the
// request's Accept-Encoding header. If two offers match with equal weight,
// the offer earlier in the list is preferred. If no offers are acceptable,
// then "" is returned.
func NegotiateContentEncoding(r *http.Request, offers []string) string {
	specs := header.ParseAccept(r.Header, "Accept-Encoding")
	if len(specs) == 0 {
		return "identity"
	}

	wildcardQ := -1.0
	for _, spec := range specs {
		if spec.Value == "*" && spec.Q > wildcardQ {
			wildcardQ = spec.Q
		}
	}

	bestOffer := ""
	bestQ := 0.0
	for _, offer := range offers {
		q := encodingQuality(specs, offer, wildcardQ)
		if q > bestQ {
			bestQ = q
			bestOffer = offer
		}
	}
	if bestOffer != "" {
		return bestOffer
	}
	if encodingQuality(specs, "identity", wildcardQ) == 0 {
		return ""
	}
	return "identity"
}

// encodingQuality returns the quality factor for encoding from specs per
// RFC 9110 Section 12.5.3. An explicit case-insensitive match takes precedence
// over the wildcard "*". It returns -1 if neither matches.
func encodingQuality(specs []header.AcceptSpec, encoding string, wildcardQ float64) float64 {
	q := -1.0
	for _, spec := range specs {
		if strings.EqualFold(spec.Value, encoding) && spec.Q > q {
			q = spec.Q
		}
	}
	if q >= 0 {
		return q
	}
	return wildcardQ
}
