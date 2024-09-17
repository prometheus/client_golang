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

//go:build arm64

package prometheus

// These are macros in xnu/osfmk/mach/shared_memory_server.h.  Note that __arm__
// is not defined for 64-bit arm.
const (
	globalSharedTextSegment uint64 = 0x90000000

	sharedTextRegionSize uint64 = 0x10000000
	sharedDataRegionSize uint64 = 0x10000000
)

const (
	machTaskBasicInfoSizeOf   = 48 // sizeof(struct mach_task_basic_info)
	vmRegionBasicInfo64SizeOf = 36 // sizeof(struct vm_region_basic_info_64)
)

// Fundamental mach types defined in xnu/osfmk/mach/arm/vm_types.h.  The integer_t and
// __darwin_natural_t types are explicitly 32-bit here to help decoding into a structure,
// where 'int' types are not supported.  The __darwin_natural_t type is defined in
// xnu/bsd/arm/_types.h.
type (
	__darwin_natural_t = uint32             // typedef unsigned int __darwin_natural_t;
	integer_t          = int32              // typedef int integer_t;
	natural_t          = __darwin_natural_t // typedef __darwin_natural_t natural_t;

	mach_vm_offset_t = uint64 // typedef uint64_t mach_vm_offset_t __kernel_ptr_semantics;
	mach_vm_size_t   = uint64 // typedef uint64_t mach_vm_size_t;
)

// Defined in xnu/osfmk/mach/arm/boolean.h, and explicitly 32-bit here to help decoding
// into a structure, where 'int' types are not supported.
type (
	boolean_t = int32 // typedef int boolean_t;
)

// Defined in xnu/osfmk/mach/arm/kern_return.h; see xnu/osfmk/mach/kern_return.h for
// possible values.
type (
	kern_return_t = int // typedef int kern_return_t;
)
