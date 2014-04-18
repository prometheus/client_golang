// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"testing"
)

type descBuildScenario struct {
	in  Desc
	out Desc
	err error
}

func (s *descBuildScenario) test(t *testing.T) {
	if err := s.in.build(); err != s.err {
		t.Fatalf("expected err %s, got %s", s.err, err)
	}
	if s.in.canonName != s.out.canonName {
		t.Fatalf("expected canonName %s, got %s", s.out.canonName, s.in.canonName)
	}
}

func TestDescBuild(t *testing.T) {
	scenarios := []descBuildScenario{
		{
			in:  Desc{},
			out: Desc{},
			err: errEmptyName,
		},
		{
			in: Desc{
				Name: "a-metric",
			},
			out: Desc{},
			err: errEmptyHelp,
		},
		{
			in: Desc{
				Name: "name",
				Help: "useless-help",
			},
			out: Desc{
				canonName: "name",
			},
			err: nil,
		},
		{
			in: Desc{
				Namespace: "namespace",
				Name:      "name",
				Help:      "useless-help",
			},
			out: Desc{
				canonName: "namespace_name",
			},
			err: nil,
		},
	}

	for _, s := range scenarios {
		s.test(t)
	}
}
