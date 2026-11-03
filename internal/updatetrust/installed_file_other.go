//go:build !linux

package updatetrust

import (
	"errors"
	"os"
)

// Release tooling also cross-compiles for Windows; installed-state admission
// runs on Linux. On other platforms refuse nonregular paths before opening.
func openVersionRecord(filename string) (*os.File, error) {
	before, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("nonregular installed version record")
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		f.Close()
		return nil, errors.New("installed version record changed")
	}
	return f, nil
}
