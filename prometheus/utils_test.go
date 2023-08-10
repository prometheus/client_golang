// Copyright 2018 The Prometheus Authors
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
package prometheus_test

import (
	"bytes"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// printlnNormalized is a helper function to compare proto messages in json format.
// Without removing brittle, we can't assert that two proto messages in json/text format are equal.
// Read more in https://github.com/golang/protobuf/issues/1121
func printlnNormalized(m proto.Message) {
	fmt.Println(protoToNormalizedJSON(m))
}

// protoToNormalizedJSON works as printlnNormalized, but returns the string instead of printing.
func protoToNormalizedJSON(m proto.Message) string {
	mAsJSON, err := protojson.Marshal(m)
	if err != nil {
		panic(err)
	}

	buffer := new(bytes.Buffer)
	if err := json.Compact(buffer, mAsJSON); err != nil {
		panic(err)
	}
	return buffer.String()
}
