# Copyright 2018 The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

include .bingo/Variables.mk
include Makefile.common

.PHONY: test
test: deps common-test

.PHONY: test-short
test-short: deps common-test-short

.PHONY: generate-go-collector-test-files
VERSIONS := 1.20 1.21 1.22
generate-go-collector-test-files:
	for GO_VERSION in $(VERSIONS); do \
		docker run \
			--platform linux/amd64 \
			--rm -v $(PWD):/workspace \
			-w /workspace \
			golang:$$GO_VERSION \
			bash ./generate-go-collector.bash; \
	done; \
	go mod tidy

.PHONY: fmt
fmt: common-format
	$(GOIMPORTS) -local github.com/prometheus/client_golang -w .
