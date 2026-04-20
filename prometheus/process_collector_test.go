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

package prometheus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPidFileFn(t *testing.T) {
	folderPath, err := os.Getwd()
	if err != nil {
		t.Error("failed to get current path")
	}
	mockPidFilePath := filepath.Join(folderPath, "mockPidFile")
	defer os.Remove(mockPidFilePath)

	testCases := []struct {
		mockPidFile       func()
		expectedErrPrefix string
		expectedPid       int
		desc              string
	}{
		{
			mockPidFile: func() {
				os.Remove(mockPidFilePath)
			},
			expectedErrPrefix: "can't read pid file",
			expectedPid:       0,
			desc:              "no existed pid file",
		},
		{
			mockPidFile: func() {
				os.Remove(mockPidFilePath)
				f, _ := os.Create(mockPidFilePath)
				f.Write([]byte("abc"))
				f.Close()
			},
			expectedErrPrefix: "can't parse pid file",
			expectedPid:       0,
			desc:              "existed pid file, error pid number",
		},
		{
			mockPidFile: func() {
				os.Remove(mockPidFilePath)
				f, _ := os.Create(mockPidFilePath)
				f.Write([]byte("123"))
				f.Close()
			},
			expectedErrPrefix: "",
			expectedPid:       123,
			desc:              "existed pid file, correct pid number",
		},
	}

	for _, tc := range testCases {
		fn := NewPidFileFn(mockPidFilePath)
		if fn == nil {
			t.Error("Should not get nil PidFileFn")
		}

		tc.mockPidFile()

		if pid, err := fn(); pid != tc.expectedPid || (err != nil && !strings.HasPrefix(err.Error(), tc.expectedErrPrefix)) {
			fmt.Println(err.Error())
			t.Error(tc.desc)
		}
	}
}
