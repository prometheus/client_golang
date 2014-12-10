package procfs

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
)

// ProcessLimits represents the soft limits for each of the process's resource
// limits.
type ProcessLimits struct {
	CPUTime          int
	FileSize         int
	DataSize         int
	StackSize        int
	CoreFileSize     int
	ResidentSet      int
	Processes        int
	OpenFiles        int
	LockedMemory     int
	AddressSpace     int
	FileLocks        int
	PendingSignals   int
	MsqqueueSize     int
	NicePriority     int
	RealtimePriority int
	RealtimeTimeout  int
}

// Limits returns the current soft limits of the process.
func (p *ProcProcess) Limits() (*ProcessLimits, error) {
	f, err := p.open("limits")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		d = regexp.MustCompile("  +")
		s = bufio.NewScanner(f)
		l ProcessLimits
	)
	for s.Scan() {
		line := s.Text()
		fields := d.Split(line, 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("couldn't parse %s line %s", f.Name(), line)
		}

		var field *int
		switch fields[0] {
		case "Max cpu time":
			field = &l.CPUTime
		case "Max file size":
			field = &l.FileLocks
		case "Max data size":
			field = &l.DataSize
		case "Max stack size":
			field = &l.StackSize
		case "Max core file size":
			field = &l.CoreFileSize
		case "Max resident set":
			field = &l.ResidentSet
		case "Max processes":
			field = &l.Processes
		case "Max open files":
			field = &l.OpenFiles
		case "Max locked memory":
			field = &l.LockedMemory
		case "Max address space":
			field = &l.AddressSpace
		case "Max file locks":
			field = &l.FileLocks
		case "Max pending signals":
			field = &l.PendingSignals
		case "Max msgqueue size":
			field = &l.MsqqueueSize
		case "Max nice priority":
			field = &l.NicePriority
		case "Max realtime priority":
			field = &l.RealtimePriority
		case "Max realtime timeout":
			field = &l.RealtimeTimeout
		default:
			continue
		}

		if fields[1] == "unlimited" {
			*field = -1
		} else {
			i, err := strconv.ParseInt(fields[1], 10, 32)
			if err != nil {
				f := "couldn't parse value %s: %s"
				return nil, fmt.Errorf(f, fields[1], err)
			}
			*field = int(i)
		}
	}

	return &l, s.Err()
}
