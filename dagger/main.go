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

// A minimal example of how to include Prometheus instrumentation.

package main

import (
	"context"
	"dagger/internal/dagger"
	"strings"

	"golang.org/x/sync/errgroup"
)

type ClientGolang struct {
	Source *dagger.Directory // +private
}

func New(src *dagger.Directory) *ClientGolang {
	return &ClientGolang{Source: src}
}

// runs `make` with the given arguments
func (m *ClientGolang) Make(
	// +optional
	args string,
	// +default="1.20"
	goVersion string,
	// +optional
	env []string,
) (string, error) {
	return dag.Golang().
		Base(goVersion).
		Container().
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithMountedCache("/go/bin", dag.CacheVolume("gobincache")).
		WithExec([]string{"sh", "-c", "make " + args}).
		Stdout(context.Background())
}

// runs `make` with the given arguments for all supported go versions
func (m *ClientGolang) MakeRun(
	ctx context.Context,
	// +optional,
	args string,
) error {
	c, err := m.Source.File("supported_go_versions.txt").Contents(ctx)
	if err != nil {
		return err
	}
	goVersions := strings.Split(c, "\n")

	eg := new(errgroup.Group)

	for _, version := range goVersions {
		version := version
		if len(version) > 0 {
			eg.Go(func() error {
				_, err := dag.Golang().
					Base(version).
					Container().
					WithMountedDirectory("/src", m.Source).
					WithWorkdir("/src").
					WithMountedCache("/go/bin", dag.CacheVolume("gobincache")).
					WithExec([]string{"sh", "-c", "make " + args}).Sync(ctx)
				return err
			})
		}
	}

	return eg.Wait()
}
