package procfs

import (
	"fmt"
	"os"
	"path"
)

// ProcFS represents the pseudo-filesystem proc, which provides an interface to
// kernel data structures.
type ProcFS struct {
	MountPoint string
}

// DefaultMountPoint is the common mount point of the filesystem.
const DefaultMountPoint = "/proc"

// NewFS returns a new ProcFS mounted under the given mountPoint. It will error
// if the mount point can't be read.
func NewFS(mountPoint string) (*ProcFS, error) {
	info, err := os.Stat(mountPoint)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %s", mountPoint, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("mount point %s is not a directory", mountPoint)
	}

	return &ProcFS{MountPoint: mountPoint}, nil
}

func (fs *ProcFS) stat(p string) (os.FileInfo, error) {
	return os.Stat(path.Join(fs.MountPoint, p))
}

func (fs *ProcFS) open(p string) (*os.File, error) {
	return os.Open(path.Join(fs.MountPoint, p))
}
