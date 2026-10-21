package peercred

import (
	"errors"
	"net"
	"syscall"
)

// UID reads the Linux SO_PEERCRED UID of the connected Unix-domain socket peer.
// Call it on the server-side accepted connection, never on an untrusted path string.
func UID(conn *net.UnixConn) (uint32, error) {
	raw, err := conn.SyscallConn()
	if err != nil { return 0, err }
	var cred *syscall.Ucred
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		cred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil { return 0, err }
	if sockErr != nil { return 0, sockErr }
	if cred == nil { return 0, errors.New("missing peer credentials") }
	return cred.Uid, nil
}

func AllowedUID(actual, expected uint32) bool { return actual == expected }
