package peercred

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestListenerAcceptAuthorized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.sock")
	underlying, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer underlying.Close()
	l := &Listener{UnixListener: underlying, ExpectedUID: uint32(os.Getuid())}
	done := make(chan error, 1)
	go func() {
		c, e := l.Accept()
		if e == nil {
			_ = c.Close()
		}
		done <- e
	}()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
