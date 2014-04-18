// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"errors"
	"strings"

	dto "github.com/prometheus/client_model/go"
)

// Metric models any sort of telemetric data you wish to export to Prometheus.
type Metric interface {
	// Desc yields the descriptor for the Metric.  Descriptors are read once by
	// Prometheus upon initial Metric registration.
	Desc() Desc
	// Write encodes the Metric into Protocol Buffer data transmission objects.
	//
	// Implementers of custom Metric types must observe concurrency safety as
	// reads of this metric may occur at any time, and any blocking occurs at
	// the expense of total performance of rendering all registered metrics.
	// Ideally Metric implementations should support concurrent readers.
	//
	// The Prometheus client library attempts to minimize memory allocations
	// and will provide a pre-existing reset dto.MetricFamily pointer.
	// Prometheus recycles the returned value, so Metric implementations should
	// not keep any reference to it.  Prometheus will never invoke Write with a
	// value.
	Write(*dto.MetricFamily)
}

// Desc is a the descriptor for all Prometheus Metrics.
type Desc struct {
	Namespace string
	Subsystem string
	Name      string

	Help string

	canonName string
}

var (
	errEmptyName             = errors.New("may not have empty name")
	errEmptyHelp             = errors.New("may not have empty help")
	errZeroCardinalityForVec = errors.New("should not use a vector type for scalar")

	errInconsistentCardinality = errors.New("inconsistent label cardinality")

	errEmptyLabelDesc = errors.New("vector may not be described by empty label dimension")
	errDuplLabelDesc  = errors.New("vector may not be described by duplicate label dimension")
)

func (d *Desc) build() error {
	if d.Name == "" {
		return errEmptyName
	}

	if d.Help == "" {
		return errEmptyHelp
	}

	switch {
	case d.Namespace != "" && d.Subsystem != "":
		d.canonName = strings.Join([]string{d.Namespace, d.Subsystem, d.Name}, "_")
		break
	case d.Namespace != "":
		d.canonName = strings.Join([]string{d.Namespace, d.Name}, "_")
		break
	case d.Subsystem != "":
		d.canonName = strings.Join([]string{d.Subsystem, d.Name}, "_")
		break
	default:
		d.canonName = d.Name
	}

	return nil
}
