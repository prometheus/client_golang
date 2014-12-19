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

const (
	limitsFields    = 3
	limitsUnlimited = "unlimited"
)

var (
	limitsDelimiter = regexp.MustCompile("  +")
)

// Limits returns the current soft limits of the process.
func (p *ProcProcess) Limits() (*ProcessLimits, error) {
	f, err := p.open("limits")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		l = ProcessLimits{}
		s = bufio.NewScanner(f)
	)
	for s.Scan() {
		line := s.Text()
		fields := limitsDelimiter.Split(line, limitsFields)
		if len(fields) != limitsFields {
			return nil, fmt.Errorf("couldn't parse %s line %s", f.Name(), line)
		}

		switch fields[0] {
		case "Max cpu time":
			l.CPUTime, err = parseInt(fields[1])
		case "Max file size":
			l.FileLocks, err = parseInt(fields[1])
		case "Max data size":
			l.DataSize, err = parseInt(fields[1])
		case "Max stack size":
			l.StackSize, err = parseInt(fields[1])
		case "Max core file size":
			l.CoreFileSize, err = parseInt(fields[1])
		case "Max resident set":
			l.ResidentSet, err = parseInt(fields[1])
		case "Max processes":
			l.Processes, err = parseInt(fields[1])
		case "Max open files":
			l.OpenFiles, err = parseInt(fields[1])
		case "Max locked memory":
			l.LockedMemory, err = parseInt(fields[1])
		case "Max address space":
			l.AddressSpace, err = parseInt(fields[1])
		case "Max file locks":
			l.FileLocks, err = parseInt(fields[1])
		case "Max pending signals":
			l.PendingSignals, err = parseInt(fields[1])
		case "Max msgqueue size":
			l.MsqqueueSize, err = parseInt(fields[1])
		case "Max nice priority":
			l.NicePriority, err = parseInt(fields[1])
		case "Max realtime priority":
			l.RealtimePriority, err = parseInt(fields[1])
		case "Max realtime timeout":
			l.RealtimeTimeout, err = parseInt(fields[1])
		}

		if err != nil {
			return nil, err
		}
	}

	return &l, s.Err()
}

func parseInt(s string) (int, error) {
	if s == limitsUnlimited {
		return -1, nil
	}
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse value %s: %s", s, err)
	}
	return int(i), nil
}
