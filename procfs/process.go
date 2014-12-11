package procfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
)

// ProcProcess provides information about a running process.
type ProcProcess struct {
	// The process ID.
	PID int

	fs *ProcFS
}

// Self returns a process for the current process.
func Self() (*ProcProcess, error) {
	return Process(os.Getpid())
}

// Process returns a process for the given pid under /proc.
func Process(pid int) (*ProcProcess, error) {
	fs, err := NewFS(DefaultMountPoint)
	if err != nil {
		return nil, err
	}

	return fs.Process(pid)
}

// Processes returns a list of all currently avaible processes under /proc.
func Processes() ([]*ProcProcess, error) {
	fs, err := NewFS(DefaultMountPoint)
	if err != nil {
		return nil, err
	}

	return fs.Processes()
}

// Process returns a process for the given pid.
func (fs *ProcFS) Process(pid int) (*ProcProcess, error) {
	if _, err := fs.stat(strconv.Itoa(pid)); err != nil {
		return nil, err
	}

	return &ProcProcess{PID: pid, fs: fs}, nil
}

// Processes returns a list of all currently avaible processes.
func (fs *ProcFS) Processes() ([]*ProcProcess, error) {
	d, err := fs.open("")
	if err != nil {
		return nil, err
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %s", d.Name(), err)
	}

	p := []*ProcProcess{}
	for _, n := range names {
		pid, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			continue
		}
		p = append(p, &ProcProcess{PID: int(pid), fs: fs})
	}

	return p, nil
}

// CmdLine returns the command line of a process.
func (p *ProcProcess) CmdLine() ([]string, error) {
	f, err := p.open("cmdline")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return strings.Split(string(data[:len(data)-1]), string(byte(0))), nil
}

// FileDescriptors returns the currently open file descriptors of a process.
func (p *ProcProcess) FileDescriptors() ([]uintptr, error) {
	names, err := p.fileDescriptors()
	if err != nil {
		return nil, err
	}

	fds := make([]uintptr, len(names))
	for i, n := range names {
		fd, err := strconv.ParseInt(n, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("could not parse fd %s: %s", n, err)
		}
		fds[i] = uintptr(fd)
	}

	return fds, nil
}

// FileDescriptorsLen returns the number of currently open file descriptors of
// a process.
func (p *ProcProcess) FileDescriptorsLen() (int, error) {
	fds, err := p.fileDescriptors()
	if err != nil {
		return 0, err
	}

	return len(fds), nil
}

func (p *ProcProcess) fileDescriptors() ([]string, error) {
	d, err := p.open("fd")
	if err != nil {
		return nil, err
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %s", d.Name(), err)
	}

	return names, nil
}

func (p *ProcProcess) open(pa string) (*os.File, error) {
	if p.fs == nil {
		return nil, fmt.Errorf("missing procfs")
	}
	return p.fs.open(path.Join(strconv.Itoa(p.PID), pa))
}
