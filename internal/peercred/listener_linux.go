package peercred

import (
	"errors"
	"net"
)

// Listener rejects unauthorized clients before net/http can serve their requests.
// All clients must connect via a Unix-domain socket.
type Listener struct {
	*net.UnixListener
	ExpectedUID uint32
}
func (l *Listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.UnixListener.AcceptUnix()
		if err != nil { return nil, err }
		uid, err := UID(conn)
		if err == nil && AllowedUID(uid, l.ExpectedUID) {
			return conn, nil
		}
		_ = conn.Close()
		if errors.Is(err, net.ErrClosed) { return nil, err }
	}
}
