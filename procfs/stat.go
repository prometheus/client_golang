package procfs

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ProcStat represents kernel/system statistics.
type ProcStat struct {
	// Boot time in seconds since the Epoch.
	BootTime int64
}

// Stat returns kernel/system statistics read from /proc/stat.
func Stat() (*ProcStat, error) {
	fs, err := NewFS(DefaultMountPoint)
	if err != nil {
		return nil, err
	}

	return fs.Stat()
}

// Stat returns an information about current kernel/system statistics.
func (fs *ProcFS) Stat() (*ProcStat, error) {
	f, err := fs.open("stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if !strings.HasPrefix(line, "btime") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("couldn't parse %s line %s", f.Name(), line)
		}
		i, err := strconv.ParseInt(fields[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse %s: %s", fields[1], err)
		}
		return &ProcStat{BootTime: i}, nil
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("couldn't parse %s: %s", f.Name(), err)
	}

	return nil, fmt.Errorf("couldn't parse %s, missing btime", f.Name())
}
