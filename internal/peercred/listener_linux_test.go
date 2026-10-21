package peercred

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestListenerRejectsUnauthorizedUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "denied.sock")
	underlying, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer underlying.Close()
	l := &Listener{UnixListener: underlying, ExpectedUID: uint32(os.Getuid()) + 1}
	done := make(chan struct{})
	go func() {
		c, err := l.Accept()
		if err == nil {
			_ = c.Close()
		}
		close(done)
	}()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if _, err := c.Read(b[:]); err != io.EOF {
		t.Fatalf("unauthorized peer must be disconnected; got %v", err)
	}
	_ = underlying.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not stop after closing")
	}
}
