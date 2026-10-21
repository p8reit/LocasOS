package peercred

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestAcceptedPeerUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peer.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	done := make(chan error, 1)
	go func() {
		c, err := net.Dial("unix", path)
		if err == nil {
			_ = c.Close()
		}
		done <- err
	}()
	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	uid, err := UID(c.(*net.UnixConn))
	if err != nil {
		t.Fatal(err)
	}
	if uid != uint32(os.Getuid()) {
		t.Fatalf("UID %d != %d", uid, os.Getuid())
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestAuthorize(t *testing.T) {
	if !AllowedUID(1000, 1000) || AllowedUID(1001, 1000) {
		t.Fatal("UID policy failure")
	}
}
