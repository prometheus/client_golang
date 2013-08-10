# Copyright 2013 Prometheus Team
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

OS   = $(shell uname)
ARCH = $(shell uname -m)

BUILD_PATH = $(PWD)/.build

export GO_VERSION = 1.1
export GOOS 	    = $(subst Darwin,darwin,$(subst Linux,linux,$(OS)))
export GOARCH		  = $(subst x86_64,amd64,$(ARCH))
export GOPKG		  = go$(GO_VERSION).$(GOOS)-$(GOARCH).tar.gz
export GOROOT		  = $(BUILD_PATH)/root/go
export GOPATH		  = $(BUILD_PATH)/root/gopath
export GOCC		    = $(GOROOT)/bin/go
export TMPDIR		  = /tmp
export GOENV		  = TMPDIR=$(TMPDIR) GOROOT=$(GOROOT) GOPATH=$(GOPATH)
export GO	        = $(GOENV) $(GOCC)
export GOFMT		  = $(GOROOT)/bin/gofmt

FULL_GOPATH = $(GOPATH)/src/github.com/prometheus/client_golang
FULL_GOPATH_BASE = $(GOPATH)/src/github.com/prometheus

MAKE_ARTIFACTS = search_index $(BUILD_PATH)

all: test

$(BUILD_PATH):
	mkdir -vp $(BUILD_PATH)

$(BUILD_PATH)/cache: $(BUILD_PATH)
	mkdir -vp $(BUILD_PATH)/cache

$(BUILD_PATH)/root: $(BUILD_PATH)
	mkdir -vp $(BUILD_PATH)/root

$(BUILD_PATH)/cache/$(GOPKG): $(BUILD_PATH)/cache
	curl -o $@ http://go.googlecode.com/files/$(GOPKG)

$(GOCC): $(BUILD_PATH)/root $(BUILD_PATH)/cache/$(GOPKG)
	tar -C $(BUILD_PATH)/root -xzf $(BUILD_PATH)/cache/$(GOPKG)
	touch $@

build: source_path dependencies
	$(MAKE) -C prometheus build
	$(MAKE) -C examples build

dependencies: source_path $(GOCC)
	$(GO) get github.com/matttproud/gocheck

test: build
	$(GO) test ./...

advice: test
	$(MAKE) -C prometheus advice
	$(MAKE) -C examples advice

format:
	find . -iname '*.go' | grep -v './.build/' | xargs -n1 -P1 $(GOFMT) -w -s=true

search_index:
	godoc -index -write_index -index_files='search_index'

# source_path is responsible for ensuring that the builder has not done anything
# stupid like working on Prometheus outside of ${GOPATH}.
source_path:
	-[ -d "$(FULL_GOPATH)" ] || { mkdir -vp $(FULL_GOPATH_BASE) ; ln -s "$(PWD)" "$(FULL_GOPATH)" ; }
	[ -d "$(FULL_GOPATH)" ]

documentation: search_index
	godoc -http=:6060 -index -index_files='search_index'

clean:
	$(MAKE) -C examples clean
	rm -rf $(MAKE_ARTIFACTS)
	find . -iname '*~' -exec rm -f '{}' ';'
	find . -iname '*#' -exec rm -f '{}' ';'

.PHONY: advice build clean documentation format source_path test
