// Copyright 2014 Prometheus Team
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
