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

// Package text contains helper functions to parse and create text-based
// exchange formats. The package currently supports (only) version 0.0.4 of the
// exchange format. Should other version be supported in the future, some
// versioning scheme has to be applied. Possibilities include separate packages
// or separate functions. The best way depends on the nature of future changes,
// which is the reason why no versioning scheme has been applied prematurely
// here.
package text

import (
	"fmt"
	"io"

	"code.google.com/p/goprotobuf/proto"
)

// WriteProtoText writes the proto.Message to the writer in text format and
// returns the number of bytes written and any error encountered.
func WriteProtoText(w io.Writer, p proto.Message) (int, error) {
	return fmt.Fprintf(w, "%s\n", proto.MarshalTextString(p))
}

// WriteProtoCompactText writes the proto.Message to the writer in compact text
// format and returns the number of bytes written and any error encountered.
func WriteProtoCompactText(w io.Writer, p proto.Message) (int, error) {
	return fmt.Fprintf(w, "%s\n", p)
}
