//go:build linux

package appmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type unitIdentity struct {
	pid     int
	entered int64
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, fmt.Errorf("local runtime observation unavailable")
	}
	return b, nil
}

type boundedUnitOutput struct{ bytes.Buffer }

func (w *boundedUnitOutput) Write(data []byte) (int, error) {
	if w.Len()+len(data) > 4096 {
		return 0, io.ErrShortWrite
	}
	return w.Buffer.Write(data)
}
func localUnit(ctx context.Context) (unitIdentity, bool) {
	cmd := exec.CommandContext(ctx, "/usr/bin/systemctl", "show", "locas-ollama.service", "--property=MainPID,ActiveState,ActiveEnterTimestampMonotonic", "--no-pager")
	var out boundedUnitOutput
	cmd.Stdout = &out
	if cmd.Run() != nil {
		return unitIdentity{}, false
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		fields := strings.SplitN(line, "=", 2)
		if len(fields) != 2 {
			return unitIdentity{}, false
		}
		if _, exists := values[fields[0]]; exists {
			return unitIdentity{}, false
		}
		values[fields[0]] = fields[1]
	}
	pid, err := strconv.Atoi(values["MainPID"])
	if err != nil || pid <= 0 || len(values) != 3 || values["ActiveState"] != "active" {
		return unitIdentity{}, false
	}
	entered, err := strconv.ParseInt(values["ActiveEnterTimestampMonotonic"], 10, 64)
	if err != nil || entered <= 0 {
		return unitIdentity{}, false
	}
	return unitIdentity{pid: pid, entered: entered}, true
}

type listenSocket struct {
	inode string
	uid   uint32
}

func parseListeners(raw []byte, port int, ipv6 bool) ([]listenSocket, bool) {
	var found []listenSocket
	for _, line := range strings.Split(string(raw), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 10 {
			return nil, false
		}
		address := strings.Split(fields[1], ":")
		if len(address) != 2 {
			return nil, false
		}
		number, err := strconv.ParseUint(address[1], 16, 16)
		if err != nil {
			return nil, false
		}
		if number != uint64(port) || fields[3] != "0A" {
			continue
		}
		// The service is IPv4 loopback-only. A wildcard/IPv6/other owner is not ours.
		if ipv6 || address[0] != "0100007F" {
			return nil, false
		}
		uid, err := strconv.ParseUint(fields[7], 10, 32)
		if err != nil {
			return nil, false
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil || inode == 0 {
			return nil, false
		}
		found = append(found, listenSocket{inode: fields[9], uid: uint32(uid)})
	}
	return found, true
}
func listeners(port int) ([]listenSocket, bool) {
	var result []listenSocket
	for _, file := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		raw, err := readBounded(file, 1<<20)
		if err != nil {
			return nil, false
		}
		part, ok := parseListeners(raw, port, strings.HasSuffix(file, "tcp6"))
		if !ok {
			return nil, false
		}
		result = append(result, part...)
	}
	return result, len(result) == 1
}
func serviceAccountSocket(info unitIdentity, port int) bool {
	identity, err := user.Lookup("locas-ollama")
	if err != nil {
		return false
	}
	uid, err := strconv.ParseUint(identity.Uid, 10, 32)
	if err != nil {
		return false
	}
	process, err := os.Stat(fmt.Sprintf("/proc/%d", info.pid))
	if err != nil {
		return false
	}
	stat, ok := process.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(uid) {
		return false
	}
	sockets, ok := listeners(port)
	if !ok || sockets[0].uid != uint32(uid) {
		return false
	}
	// Prevent transient Type=simple activation from borrowing a foreign API during
	// the observed bind-failure/restart loop. This is account-level readiness,
	// explicitly not exact PID/FD attribution for a same-UID foreign process.
	raw, err := readBounded("/proc/uptime", 256)
	if err != nil {
		return false
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 {
		return false
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	return err == nil && seconds > 0 && seconds < 100*365*24*3600 && int64(seconds*1e6)-info.entered >= 5_000_000
}
func exactProcessSocket(info unitIdentity, port int) bool {
	sockets, ok := listeners(port)
	if !ok {
		return false
	}
	directory, err := os.Open(fmt.Sprintf("/proc/%d/fd", info.pid))
	if err != nil {
		return false
	}
	defer directory.Close()
	entries, err := directory.ReadDir(4097)
	if err != nil && err != io.EOF {
		return false
	}
	if len(entries) > 4096 {
		return false
	}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(directory.Name(), entry.Name()))
		if err == nil && target == "socket:["+sockets[0].inode+"]" {
			return true
		}
	}
	return false
}
