package procfs

import (
	"os"
	"reflect"
	"testing"
)

func TestSelf(t *testing.T) {
	p1, err := Process(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Self()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(p1, p2) {
		t.Errorf("want process %v to equal %v", p1, p2)
	}
}

func TestProcesses(t *testing.T) {
	fs, err := NewFS("fixtures")
	if err != nil {
		t.Fatal(err)
	}
	procs, err := fs.Processes()
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range []*Proc{{PID: 584}, {PID: 26231}} {
		if want, got := p.PID, procs[i].PID; want != got {
			t.Errorf("want processes %d, got %d", want, got)
		}
	}
}

func TestCmdLine(t *testing.T) {
	p1, err := testProcess(26231)
	if err != nil {
		t.Fatal(err)
	}
	c, err := p1.CmdLine()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"vim", "test.go", "+10"}; !reflect.DeepEqual(want, c) {
		t.Errorf("want cmdline %v, got %v", want, c)
	}
}

func TestFileDescriptors(t *testing.T) {
	p1, err := testProcess(26231)
	if err != nil {
		t.Fatal(err)
	}
	fds, err := p1.FileDescriptors()
	if err != nil {
		t.Fatal(err)
	}
	if want := []uintptr{2, 4, 1, 3, 0}; !reflect.DeepEqual(want, fds) {
		t.Errorf("want fds %v, got %v", want, fds)
	}

	p2, err := Self()
	if err != nil {
		t.Fatal(err)
	}

	fdsBefore, err := p2.FileDescriptors()
	if err != nil {
		t.Fatal(err)
	}

	s, err := os.Open("fixtures")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	fdsAfter, err := p2.FileDescriptors()
	if err != nil {
		t.Fatal(err)
	}

	if len(fdsBefore)+1 != len(fdsAfter) {
		t.Errorf("want fds %v+1 to equal %v", fdsBefore, fdsAfter)
	}
}

func TestFileDescriptorsLen(t *testing.T) {
	p1, err := testProcess(26231)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p1.FileDescriptorsLen()
	if err != nil {
		t.Fatal(err)
	}
	if want, got := 5, l; want != got {
		t.Errorf("want fds %d, got %d", want, got)
	}
}

func testProcess(pid int) (*Proc, error) {
	fs, err := NewFS("fixtures")
	if err != nil {
		return nil, err
	}

	return fs.Process(pid)
}
