//go:build linux

package updatetrust

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestVersionRecordFIFORejectedWithoutBlocking(t *testing.T) {
	file := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(file, 0600); err != nil {
		t.Fatal(err)
	}
	if InspectInstalled("dev", file).Basis != "invalid_state" {
		t.Fatal("FIFO treated as version state")
	}
}
