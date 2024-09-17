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

package purego

/*
size_t getGlobalSharedTextSegment();
size_t getSharedTextRegionSize();
size_t getSharedDataRegionSize();

size_t getSizeofMachTaskBasicInfo();
size_t getOffsetOfMachTaskBasicInfo_virtual_size();
size_t getSizeofMachTaskBasicInfo_virtual_size();
size_t getOffsetOfMachTaskBasicInfo_resident_size();
size_t getSizeofMachTaskBasicInfo_resident_size();

size_t getSizeofVmRegionBasicInfo64();
size_t getOffsetOfVmRegionBasicInfo64_reserved();
size_t getSizeofVmRegionBasicInfo64_reserved();
*/
import "C"

func GetGlobalSharedTextSegment() uint64 {
	return uint64(C.getGlobalSharedTextSegment())
}

func GetSharedTextRegionSize() uint64 {
	return uint64(C.getSharedTextRegionSize())
}

func GetSharedDataRegionSize() uint64 {
	return uint64(C.getSharedDataRegionSize())
}

func GetSizeofMachTaskBasicInfo() uint64 {
	return uint64(C.getSizeofMachTaskBasicInfo())
}

func GetOffsetOfMachTaskBasicInfo_virtual_size() uint64 {
	return uint64(C.getOffsetOfMachTaskBasicInfo_virtual_size())
}

func GetSizeofMachTaskBasicInfo_virtual_size() uint64 {
	return uint64(C.getSizeofMachTaskBasicInfo_virtual_size())
}

func GetOffsetOfMachTaskBasicInfo_resident_size() uint64 {
	return uint64(C.getOffsetOfMachTaskBasicInfo_resident_size())
}

func GetSizeofMachTaskBasicInfo_resident_size() uint64 {
	return uint64(C.getSizeofMachTaskBasicInfo_resident_size())
}

func GetSizeofVmRegionBasicInfo64() uint64 {
	return uint64(C.getSizeofVmRegionBasicInfo64())
}

func GetOffsetOfVmRegionBasicInfo64_reserved() uint64 {
	return uint64(C.getOffsetOfVmRegionBasicInfo64_reserved())
}

func GetSizeofVmRegionBasicInfo64_reserved() uint64 {
	return uint64(C.getSizeofVmRegionBasicInfo64_reserved())
}
