// Package auditlog provides bounded, durable, per-service security journals.
// Its hash chain detects corruption, not a compromised writer or root: a full
// rewrite, deletion, or valid-prefix truncation requires an external anchor to detect.
package auditlog

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

const DefaultCapacity int64 = 16 << 20
const MaxEntryBytes = 4096
const maxLine = MaxEntryBytes
const journalName = "events.jsonl"

type Recorder interface{ Append(Event) error }

// Event deliberately permits no arbitrary text, credentials, paths, or bodies.
// SessionID is a separately generated audit reference, never a session credential.
type Event struct {
	CorrelationID string  `json:"correlation_id"`
	Service       string  `json:"service"`
	Operation     string  `json:"operation"`
	Stage         string  `json:"stage"`
	Principal     string  `json:"principal"`
	SessionID     string  `json:"session_id,omitempty"`
	Source        string  `json:"source"`
	PeerUID       *uint32 `json:"peer_uid,omitempty"`
	Outcome       string  `json:"outcome"`
	HTTPStatus    int     `json:"http_status,omitempty"`
	App           string  `json:"app,omitempty"`
	Version       string  `json:"version,omitempty"`
	Channel       string  `json:"channel,omitempty"`
	SHA256        string  `json:"sha256,omitempty"`
}

type record struct {
	WriterUID    uint32 `json:"writer_uid"`
	Sequence     uint64 `json:"sequence"`
	Time         string `json:"time"`
	PreviousHash string `json:"previous_hash"`
	Event        Event  `json:"event"`
	Hash         string `json:"hash"`
}

type Report struct {
	history   map[string]bool
	active    map[string]bool
	Service   string   `json:"service"`
	Entries   uint64   `json:"entries"`
	LastHash  string   `json:"last_hash"`
	Pending   []string `json:"pending"`
	Uncertain []string `json:"uncertain"`
}

type Journal struct {
	dir      string
	service  string
	mu       sync.Mutex
	poisoned bool
	capacity int64
	// Private fault-injection hooks keep production operations unchanged.
	write    func(*os.File, []byte) (int, error)
	syncFile func(*os.File) error
}

func member(s string, allowed ...string) bool {
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	return false
}
func hexString(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func serviceOK(s string) bool { return member(s, "core", "agent", "updater", "app-worker") }
func validate(e Event) error {
	if !hexString(e.CorrelationID, 32) || !serviceOK(e.Service) || !member(e.Operation, "auth.setup", "auth.login", "auth.logout", "model.pull", "model.chat", "app.install", "update.channel", "update.discover", "update.install", "ipc.connect", "request.denied") || !member(e.Stage, "intent", "result", "verified", "apply") || !member(e.Principal, "anonymous", "administrator", "system") || !member(e.Source, "browser", "core_ipc", "agent_queue", "system") || !member(e.Outcome, "pending", "allowed", "denied", "accepted", "succeeded", "failed", "uncertain") {
		return errors.New("invalid audit event classification")
	}
	if e.SessionID != "" && !hexString(e.SessionID, 32) || e.SHA256 != "" && !hexString(e.SHA256, 64) || e.HTTPStatus != 0 && (e.HTTPStatus < 100 || e.HTTPStatus > 599) || e.App != "" && !member(e.App, "ollama", "open-webui") || e.Channel != "" && !member(e.Channel, "stable", "beta") {
		return errors.New("invalid audit event attribute")
	}
	if e.Version != "" {
		if _, ok := updatetrust.VersionParts(e.Version); !ok {
			return errors.New("invalid audit version")
		}
	}
	return nil
}

// openDirectory traverses every component without following symbolic links.
func openDirectory(path string, create bool) (*os.File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(abs, "/"), "/") {
		if part == "" {
			continue
		}
		next, e := syscall.Openat(fd, part, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if e == syscall.ENOENT && create {
			e = syscall.Mkdirat(fd, part, 0700)
			if e == nil {
				e = syscall.Fsync(fd)
			}
			if e == nil || e == syscall.EEXIST {
				next, e = syscall.Openat(fd, part, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
			}
		}
		syscall.Close(fd)
		if e != nil {
			return nil, errors.New("unsafe or unavailable audit directory")
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), abs)
	var st syscall.Stat_t
	if err = syscall.Fstat(fd, &st); err != nil || st.Uid != uint32(os.Geteuid()) || st.Mode&07777 != 0700 {
		f.Close()
		return nil, errors.New("audit directory must be owned by current UID with mode 0700")
	}
	return f, nil
}

func openLog(d *os.File, create, writable bool) (*os.File, error) {
	flags := syscall.O_RDWR | syscall.O_APPEND
	if !writable {
		flags = syscall.O_RDONLY
	}
	fd, err := syscall.Openat(int(d.Fd()), journalName, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err == syscall.ENOENT && create {
		fd, err = syscall.Openat(int(d.Fd()), journalName, flags|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
		if err == nil {
			if e := syscall.Fsync(fd); e != nil {
				syscall.Close(fd)
				return nil, e
			}
			if e := d.Sync(); e != nil {
				syscall.Close(fd)
				return nil, e
			}
		}
	}
	if err != nil {
		return nil, errors.New("cannot open audit journal")
	}
	f := os.NewFile(uintptr(fd), journalName)
	var st syscall.Stat_t
	if err = syscall.Fstat(fd, &st); err != nil || st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Mode&07777 != 0600 || st.Uid != uint32(os.Geteuid()) || st.Nlink != 1 {
		f.Close()
		return nil, errors.New("unsafe audit journal ownership, type, links, or permissions")
	}
	return f, nil
}

func hashRecord(r record) string {
	r.Hash = ""
	b, _ := json.Marshal(r)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func inspect(f *os.File, capacity int64) (Report, error) {
	report := Report{Pending: []string{}, Uncertain: []string{}, history: map[string]bool{}, active: map[string]bool{}}
	st, err := f.Stat()
	if err != nil {
		return report, err
	}
	if st.Size() > capacity {
		return report, errors.New("audit journal exceeds capacity")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return report, err
	}
	reader := bufio.NewReaderSize(io.LimitReader(f, capacity+1), maxLine+1)
	pending := map[string]bool{}
	uncertain := map[string]bool{}
	for {
		line, e := reader.ReadSlice('\n')
		if e == io.EOF && len(line) == 0 {
			break
		}
		if e != nil || len(line) > maxLine {
			return report, errors.New("incomplete or oversized audit record")
		}
		var r record
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if e = dec.Decode(&r); e != nil {
			return report, errors.New("invalid audit record JSON")
		}
		if dec.Decode(new(any)) != io.EOF {
			return report, errors.New("trailing audit record data")
		}
		canonical, _ := json.Marshal(r)
		if !bytes.Equal(append(canonical, '\n'), line) {
			return report, errors.New("noncanonical audit record")
		}
		if r.WriterUID != uint32(os.Geteuid()) || validate(r.Event) != nil || r.Sequence != report.Entries+1 || r.PreviousHash != report.LastHash || !hexString(r.Hash, 64) || hashRecord(r) != r.Hash {
			return report, errors.New("audit integrity verification failed")
		}
		if t, e := time.Parse(time.RFC3339Nano, r.Time); e != nil || t.Location() != time.UTC {
			return report, errors.New("invalid audit timestamp")
		}
		if report.Service == "" {
			report.Service = r.Event.Service
		}
		if r.Event.Service != report.Service {
			return report, errors.New("mixed audit services")
		}
		report.Entries = r.Sequence
		report.LastHash = r.Hash
		id := r.Event.CorrelationID
		key := id + ":" + r.Event.Operation
		switch r.Event.Stage {
		case "intent":
			if report.history[key] {
				return report, errors.New("duplicate audit intent")
			}
			report.history[key] = true
			report.active[key] = true
		case "result":
			if !report.active[key] {
				return report, errors.New("audit result without intent")
			}
			delete(report.active, key)
		default:
			if !report.active[key] {
				return report, errors.New("audit stage without intent")
			}
		}

		if r.Event.Outcome == "uncertain" {
			uncertain[key] = true
		} else if r.Event.Stage == "result" {
			delete(uncertain, key)
		}
	}
	for key := range report.active {
		pending[strings.SplitN(key, ":", 2)[0]] = true
	}
	for id := range pending {
		report.Pending = append(report.Pending, id)
	}
	uncertainIDs := map[string]bool{}
	for key := range uncertain {
		uncertainIDs[strings.SplitN(key, ":", 2)[0]] = true
	}
	for id := range uncertainIDs {
		report.Uncertain = append(report.Uncertain, id)
	}
	sort.Strings(report.Pending)
	sort.Strings(report.Uncertain)
	return report, nil
}

func Open(dir, service string) (*Journal, error) {
	if !serviceOK(service) {
		return nil, errors.New("invalid audit service")
	}
	j := &Journal{dir: dir, service: service, capacity: DefaultCapacity, write: func(f *os.File, b []byte) (int, error) { return f.Write(b) }, syncFile: func(f *os.File) error { return f.Sync() }}
	err := j.withLog(true, func(f *os.File) error {
		r, e := inspect(f, j.capacity)
		if e == nil && r.Service != "" && r.Service != service {
			return errors.New("audit service mismatch")
		}
		return e
	})
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (j *Journal) withLog(create bool, fn func(*os.File) error) error {
	d, err := openDirectory(j.dir, create)
	if err != nil {
		return err
	}
	defer d.Close()
	if err = syscall.Flock(int(d.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(d.Fd()), syscall.LOCK_UN)
	f, err := openLog(d, create, true)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(f)
}

func (j *Journal) Append(e Event) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.poisoned {
		return errors.New("audit journal poisoned after persistence failure")
	}
	if err := validate(e); err != nil {
		return err
	}
	if e.Service != j.service {
		return errors.New("audit service mismatch")
	}
	// Append never recreates a removed log or directory after Open.
	return j.withLog(false, func(f *os.File) error {
		r, err := inspect(f, j.capacity)
		if err != nil {
			return err
		}
		if r.Service != "" && r.Service != j.service {
			return errors.New("audit service mismatch")
		}
		key := e.CorrelationID + ":" + e.Operation
		reservations := len(r.active)
		if e.Stage == "intent" {
			if r.history[key] {
				return errors.New("duplicate audit intent")
			}
			reservations++
		} else if !r.active[key] {
			return errors.New("audit stage without intent")
		} else if e.Stage == "result" {
			reservations--
		}
		entry := record{WriterUID: uint32(os.Geteuid()), Sequence: r.Entries + 1, Time: time.Now().UTC().Format(time.RFC3339Nano), PreviousHash: r.LastHash, Event: e}
		entry.Hash = hashRecord(entry)
		b, _ := json.Marshal(entry)
		b = append(b, '\n')
		st, err := f.Stat()
		if err != nil {
			return err
		}
		if len(b) > maxLine || st.Size()+int64(len(b))+int64(reservations)*MaxEntryBytes > j.capacity {
			return errors.New("audit capacity exhausted")
		}
		n, err := j.write(f, b)
		if err == nil && n != len(b) {
			err = io.ErrShortWrite
		}
		if err == nil {
			err = j.syncFile(f)
		}
		if err != nil {
			j.poisoned = true
			return fmt.Errorf("audit persistence failed: %w", err)
		}
		return nil
	})
}

// Verify opens existing files read-only and never creates or repairs state.
func Verify(dir string) (Report, error) {
	d, err := openDirectory(dir, false)
	if err != nil {
		return Report{}, err
	}
	defer d.Close()
	if err = syscall.Flock(int(d.Fd()), syscall.LOCK_SH); err != nil {
		return Report{}, err
	}
	defer syscall.Flock(int(d.Fd()), syscall.LOCK_UN)
	f, err := openLog(d, false, false)
	if err != nil {
		return Report{}, err
	}
	defer f.Close()
	return inspect(f, DefaultCapacity)
}
