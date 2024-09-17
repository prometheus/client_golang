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

#include <mach/mach_init.h>
#include <mach/task.h>
// Compiler warns shared_memory_server.h is deprecated, use this instead.
// But this doesn't define SHARED_DATA_REGION_SIZE and SHARED_TEXT_REGION_SIZE.
//#include <mach/shared_region.h>
#include <mach/shared_memory_server.h>
#include <mach/mach_vm.h>
#include <stddef.h>


size_t getGlobalSharedTextSegment()
{
    return GLOBAL_SHARED_TEXT_SEGMENT;
}
size_t getSharedTextRegionSize()
{
    return SHARED_TEXT_REGION_SIZE;
}
size_t getSharedDataRegionSize()
{
    return SHARED_DATA_REGION_SIZE;
}

size_t getSizeofMachTaskBasicInfo()
{
    return sizeof(struct mach_task_basic_info);
}

size_t getOffsetOfMachTaskBasicInfo_virtual_size()
{
    return offsetof(struct mach_task_basic_info, virtual_size);
}

size_t getSizeofMachTaskBasicInfo_virtual_size()
{
    struct mach_task_basic_info info;
    return sizeof(info.virtual_size);
}

size_t getOffsetOfMachTaskBasicInfo_resident_size()
{
    return offsetof(struct mach_task_basic_info, resident_size);
}

size_t getSizeofMachTaskBasicInfo_resident_size()
{
    struct mach_task_basic_info info;
    return sizeof(info.resident_size);
}

size_t getSizeofVmRegionBasicInfo64()
{
    return sizeof(struct vm_region_basic_info_64);
}

size_t getOffsetOfVmRegionBasicInfo64_reserved()
{
    return offsetof(struct vm_region_basic_info_64, reserved);
}

size_t getSizeofVmRegionBasicInfo64_reserved()
{
    struct vm_region_basic_info_64 data;
    return sizeof(data.reserved);
}
