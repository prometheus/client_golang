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

package prometheus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/ebitengine/purego"
)

func init() {
	if lib, err := purego.Dlopen("/usr/lib/system/libsystem_kernel.dylib",
		purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {

		// purego.RegisterLibFunc() panics if the symbol is missing.  Ignore any error,
		// and the metric will simply be unavailable when it is queried, instead of
		// bringing down the whole process.
		defer func() {
			if err := recover(); err != nil {
				// TODO: Log this somehow
			}
		}()

		purego.RegisterLibFunc(&machTaskSelf, lib, "mach_task_self")
		purego.RegisterLibFunc(&taskInfo, lib, "task_info")
		purego.RegisterLibFunc(&machVmRegion, lib, "mach_vm_region")
	}
}

const (
	// The task_info() flavor MACH_TASK_BASIC_INFO for retrieving machTaskBasicInfo.
	mach_task_basic_info task_flavor_t = 20 /* always 64-bit basic info */

	// The MACH_TASK_BASIC_INFO_COUNT value, which is passed to the Mach API as the size
	// of the payload for MACH_TASK_BASIC_INFO commands.
	machTaskBasicInfoCount mach_msg_type_number_t = machTaskBasicInfoSizeOf / 4
)

// Defined in xnu/osfmk/mach/policy.h, xnu/iokit/IOKit/IORPC.h, and xnu/osfmk/mach/message.h
// respectively.  policy_t is explicitly 32-bit here to help decoding into a structure,
// where 'int' types are not supported.
type (
	policy_t               = int32     // typedef int policy_t;
	mach_port_t            = natural_t // typedef natural_t mach_port_t;
	mach_msg_type_number_t = natural_t // typedef natural_t mach_msg_type_number_t;
)

// Defined in xnu/osfmk/mach/task_info.h.  Note that task_info_t is actually defined as
// integer_t* and cast from the address of the structure in C.  Define it as []byte to
// keep the type system happy.
type (
	task_flavor_t = natural_t // typedef natural_t task_flavor_t
	task_info_t   = []byte    // typedef integer_t *task_info_t /* varying array of int */
)

// time_value_t is the type for kernel time values, defined in xnu/osfmk/mach/time_value.h
type time_value_t struct {
	Seconds      integer_t
	MicroSeconds integer_t
}

var machTaskSelf func() mach_port_t

var taskInfo func(
	mach_port_t,
	task_flavor_t,
	task_info_t,
	*mach_msg_type_number_t,
) kern_return_t

// machTaskBasicInfo is the representation of `struct mach_task_basic_info` defined in
// xnu/osfmk/mach/task_info.h, which is the architecture independent payload for fetching
// certain task values.
type machTaskBasicInfo struct {
	VirtualSize     mach_vm_size_t // virtual memory size (bytes)
	ResidentSize    mach_vm_size_t // resident memory size (bytes)
	ResidentSizeMax mach_vm_size_t // maximum resident memory size (bytes)
	UserTime        time_value_t   // total user run time for terminated threads
	SystemTime      time_value_t   // total system run time for terminated threads
	Policy          policy_t       // default policy for new threads
	SuspendCount    integer_t      // suspend count for task
}

func getBasicTaskInfo() (*machTaskBasicInfo, error) {
	var info machTaskBasicInfo

	if taskInfo == nil {
		return nil, fmt.Errorf("task_info() is not supported")
	}

	var count = machTaskBasicInfoCount
	buf := make([]byte, machTaskBasicInfoSizeOf)

	if ret := taskInfo(machTaskSelf(), mach_task_basic_info, buf, &count); ret != 0 {
		return nil, fmt.Errorf("task_info() returned %d", ret)
	}

	if err := loadStruct(buf, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// Defined in xnu/osfmk/mach/memory_object_types.h, xnu/osfmk/mach/vm_behavior.h,
// defined in xnu/osfmk/mach/vm_inherit.h, and defined in xnu/osfmk/mach/vm_prot.h
// respectively.  These types are not identical to the native definitions, because the
// struct decoding requires primitives with a specific width.  The widths here are the
// same as the native types.
type (
	memory_object_offset_t = uint64 // typedef unsigned long long memory_object_offset_t;
	vm_behavior_t          = int32  // typedef int vm_behavior_t
	vm_inherit_t           = uint32 // typedef unsigned int vm_inherit_t;
	vm_prot_t              = int32  // typedef int vm_prot_t;
)

// These are defined in xnu/osfmk/mach/vm_types.h.
type (
	vm_map_t = mach_port_t // typedef mach_port_t vm_map_t;
)

// Defined in xnu/osfmk/mach/vm_region.h.  vm_region_flavor_t is explicitly 32-bit here to
// help decoding into a structure, where 'int' types are not supported, and vm_region_info_t
// is []byte to keep the type system happy.
type (
	vm_region_flavor_t = int32  // typedef int vm_region_flavor_t;
	vm_region_info_t   = []byte // typedef int *vm_region_info_t;
)

const (
	// The mach_vm_region() flavor VM_REGION_BASIC_INFO_64 for retrieving
	// vmRegionBasicInfo64.
	vm_region_basic_info_64 vm_region_flavor_t = 9

	// The VM_REGION_BASIC_INFO_COUNT_64 value, which is passed to the Mach API as the
	// size of the payload for VM_REGION_BASIC_INFO_64 commands.
	vmRegionBasicInfoCount64 mach_msg_type_number_t = vmRegionBasicInfo64SizeOf / 4
)

// vmRegionBasicInfo64 is the representation of `struct vm_region_basic_info_64` defined
// in xnu/osfmk/mach/vm_region.h.  This is enclosed in `#pragma pack(push, 4)` in C, so
// unsafe.SizeOf() won't match the actual sizeof(vm_region_basic_info_64).
type vmRegionBasicInfo64 struct {
	Protection     vm_prot_t
	MaxProtection  vm_prot_t
	Inheritance    vm_inherit_t
	Shared         boolean_t
	Reserved       boolean_t
	Offset         memory_object_offset_t
	Behavior       vm_behavior_t
	UserWiredCount uint16
}

var machVmRegion func(
	vm_map_t,
	*mach_vm_offset_t, /* IN/OUT */
	*mach_vm_size_t, /* OUT */
	vm_region_flavor_t, /* IN */
	vm_region_info_t, /* OUT */
	*mach_msg_type_number_t, /* IN/OUT */
	*mach_port_t, /* OUT */
) kern_return_t

func getMemoryUsage() (*machTaskBasicInfo, error) {
	// The logic in here follows how the ps(1) utility determines the memory values.  The
	// basic_task_info command used here is a more modern, cross-architecture one that is
	// suggested in the kernel header files.
	//
	// https://github.com/apple-oss-distributions/adv_cmds/blob/8744084ea0ff41ca4bb96b0f9c22407d0e48e9b7/ps/tasks.c#L132

	info, err := getBasicTaskInfo()

	if err != nil {
		return nil, err
	} else if machVmRegion != nil {

		var textInfo vmRegionBasicInfo64

		buf := make([]byte, vmRegionBasicInfo64SizeOf)
		address := globalSharedTextSegment
		var size mach_vm_size_t
		var objectName mach_port_t

		cmd := vm_region_basic_info_64
		count := vmRegionBasicInfoCount64

		ret := machVmRegion(machTaskSelf(), &address, &size, cmd, buf, &count, &objectName)

		if ret == 0 {
			if err := loadStruct(buf, &textInfo); err == nil {
				adjustment := sharedTextRegionSize + sharedDataRegionSize
				if textInfo.Reserved != 0 && size == sharedTextRegionSize && info.VirtualSize > adjustment {
					info.VirtualSize -= adjustment
				}
			}
		}
	}

	return info, nil
}

func loadStruct(buffer []byte, data any) error {
	r := bytes.NewReader(buffer)

	// TODO: NativeEndian was added in go 1.21
	return binary.Read(r, binary.LittleEndian, data)
}
