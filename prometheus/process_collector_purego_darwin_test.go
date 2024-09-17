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

//go:build cgo && darwin

package prometheus

import (
	"bytes"
	"encoding/binary"
	"github.com/prometheus/client_golang/prometheus/testutil/purego"
	"math"
	"reflect"
	"testing"
)

// TestNativeTypeMatches ensures that const values for preprocessor macros, native struct
// sizes, and field offsets are in sync with the native code.
func TestNativeTypeMatches(t *testing.T) {
	tests := []struct {
		name string
		want uint64
		got  func() uint64
	}{
		{
			"sizeof(machTaskBasicInfo)",
			machTaskBasicInfoSizeOf,
			purego.GetSizeofMachTaskBasicInfo,
		},
		{
			"sizeof(machTaskBasicInfo.VirtualSize)",
			8,
			purego.GetSizeofMachTaskBasicInfo_virtual_size,
		},
		{
			"sizeof(vmRegionBasicInfo64.ResidentSize)",
			8,
			purego.GetSizeofMachTaskBasicInfo_resident_size,
		},
		{
			"sizeof(vmRegionBasicInfo64)",
			vmRegionBasicInfo64SizeOf,
			purego.GetSizeofVmRegionBasicInfo64,
		},
		{
			"sizeof(vmRegionBasicInfo64.Reserved)",
			4,
			purego.GetSizeofVmRegionBasicInfo64_reserved,
		},
		{
			"value of globalSharedTextSegment",
			globalSharedTextSegment,
			purego.GetGlobalSharedTextSegment,
		},
		{
			"value of sharedTextRegionSize",
			sharedTextRegionSize,
			purego.GetSharedTextRegionSize,
		},
		{
			"value of sharedDataRegionSize",
			sharedDataRegionSize,
			purego.GetSharedDataRegionSize,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.got()
			if test.want != got {
				t.Errorf("Expected %d, got %d\n", test.want, got)
			}
		})
	}
}

// TestNativeStructMapping ensures that fields of a proper size and at the proper offset
// in a byte array can properly initialize a corresponding structure.  Since the field
// offset in a Go struct may not match the offset in the C struct, the most robust test
// is to ensure that a field value written to a specific byte buffer offset is mapped to
// the corresponding Go struct field.  The value should be the maximum, to ensure the
// entire byte range is mapped to the structure field.
func TestNativeStructMapping(t *testing.T) {
	tests := []struct {
		name       string        // The name of the test
		field      string        // The structure field to test
		value      reflect.Value // The value of the field to set and validate
		offset     func() uint64 // The offset of the field in the native struct
		fieldSize  func() uint64 // The native width of the field in the struct
		structSize func() uint64 // The native size of the C structure
		dataType   reflect.Type
	}{
		{
			"machTaskBasicInfo_VirtualSize",
			"VirtualSize",
			reflect.ValueOf(mach_vm_size_t(math.MaxUint64)),
			purego.GetOffsetOfMachTaskBasicInfo_virtual_size,
			purego.GetSizeofMachTaskBasicInfo_virtual_size,
			purego.GetSizeofMachTaskBasicInfo,
			reflect.TypeOf(machTaskBasicInfo{}),
		},
		{
			"machTaskBasicInfo_ResidentSize",
			"ResidentSize",
			reflect.ValueOf(mach_vm_size_t(math.MaxUint64)),
			purego.GetOffsetOfMachTaskBasicInfo_resident_size,
			purego.GetSizeofMachTaskBasicInfo_resident_size,
			purego.GetSizeofMachTaskBasicInfo,
			reflect.TypeOf(machTaskBasicInfo{}),
		},
		{
			"vmRegionBasicInfo64_Reserved",
			"Reserved",
			reflect.ValueOf(boolean_t(math.MaxInt32)),
			purego.GetOffsetOfVmRegionBasicInfo64_reserved,
			purego.GetSizeofVmRegionBasicInfo64_reserved,
			purego.GetSizeofVmRegionBasicInfo64,
			reflect.TypeOf(vmRegionBasicInfo64{}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			offset := test.offset()
			fieldSize := test.fieldSize()
			structSize := test.structSize()

			buf := bytes.NewBuffer(make([]byte, 0, structSize))
			order := binary.LittleEndian

			// Move the position to the offset of the field
			for i := uint64(0); i < offset; i++ {
				var pad byte = 0
				if err := binary.Write(buf, order, pad); err != nil {
					t.Error(err)
					return
				}
			}

			// Write the test value at the desired offset.
			if err := binary.Write(buf, order, test.value.Interface()); err != nil {
				t.Error(err)
				return
			}

			// Zero fill the rest of the buffer to avoid a premature EOF.
			for i := offset + fieldSize; i < structSize; i++ {
				var pad byte = 0
				if err := binary.Write(buf, order, pad); err != nil {
					t.Error(err)
					return
				}
			}

			// Instantiate a new structure, decode the buffer into it, and then compare
			// the structure field to the expected value.
			ptr := reflect.New(test.dataType)

			if err := binary.Read(buf, order, ptr.Interface()); err != nil {
				t.Error(err)
				return
			}

			field := ptr.Elem().FieldByName(test.field)

			if field.IsZero() {
				t.Errorf("Missing field %s\n", test.field)
				return
			}

			if field.CanInt() {
				got := field.Int()

				if got != test.value.Int() {
					t.Errorf("Got %d, wanted %s\n", got, test.value)
					return
				}
			} else if field.CanUint() {
				got := field.Uint()

				if got != test.value.Uint() {
					t.Errorf("Got %d, wanted %s\n", got, test.value)
					return
				}
			} else {
				t.Errorf("Unhandled field type: %s\n", field.Type())
			}
		})
	}
}

func TestSyscall(t *testing.T) {
	// There's not a good way to validate that the value returned from the syscall is
	// accurate, but we can ensure that the function pointer is non-nil, and that no
	// error is returned.

	if taskInfo == nil {
		t.Errorf("No task_info() method found\n")
	}

	if _, err := getMemoryUsage(); err != nil {
		t.Errorf("getMemoryUsage() failed with %v\n", err)
	}
}
