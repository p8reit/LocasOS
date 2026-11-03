//go:build linux

package jobqueue

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

const databaseName = "queue.db"
const schema = `CREATE TABLE jobs (sequence INTEGER PRIMARY KEY, id TEXT NOT NULL UNIQUE, kind TEXT NOT NULL, correlation_id TEXT NOT NULL, session_id TEXT NOT NULL, memory_mib INTEGER NOT NULL, queue_timeout INTEGER NOT NULL, run_timeout INTEGER NOT NULL, status TEXT NOT NULL, queued_at INTEGER NOT NULL, started_at INTEGER NOT NULL, finished_at INTEGER NOT NULL, queue_deadline INTEGER NOT NULL, run_deadline INTEGER NOT NULL) STRICT`
const columns = "sequence,id,kind,correlation_id,session_id,memory_mib,queue_timeout,run_timeout,status,queued_at,started_at,finished_at,queue_deadline,run_deadline"

type Store struct {
	mu       sync.Mutex
	db       *sql.DB
	lock     *os.File
	dir      string
	poisoned bool
	// Replaced only by package tests to inject a failure after real SQL commit.
	checkpoint func() error
}

// Open requires a dedicated private directory, retains a lifetime exclusive
// flock, validates existing state, and marks previously running jobs interrupted.
// It never migrates, deletes, repairs, or replays jobs.
func Open(dir string) (_ *Store, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = prepareDirectory(dir); err != nil {
		return nil, err
	}
	_, lockErr := os.Lstat(filepath.Join(dir, "owner.lock"))
	hadOwner := lockErr == nil
	if lockErr != nil && !os.IsNotExist(lockErr) {
		return nil, lockErr
	}
	lock, err := privateFile(filepath.Join(dir, "owner.lock"))
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrOwned
		}
		return nil, err
	}
	s := &Store{lock: lock, dir: dir}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	if err = s.checkFiles(); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, databaseName)
	_, statErr := os.Lstat(path)
	fresh := os.IsNotExist(statErr)
	if statErr != nil && !fresh {
		return nil, statErr
	}
	// A retained owner marker or sidecar is evidence of prior initialization.
	// Never turn missing database bytes into an apparently empty history.
	if fresh {
		if hadOwner {
			return nil, ErrInvalid
		}
		for _, suffix := range []string{"-wal", "-shm"} {
			if _, e := os.Lstat(path + suffix); e == nil {
				return nil, ErrInvalid
			} else if !os.IsNotExist(e) {
				return nil, e
			}
		}
	}
	f, err := privateFile(path)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	f.Close()
	if err != nil {
		return nil, err
	}
	if !fresh && stat.Size() == 0 {
		return nil, ErrInvalid
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecar, e := privateFile(path + suffix)
		if e != nil {
			return nil, e
		}
		sidecar.Close()
	}
	u := url.URL{Scheme: "file", Path: path}
	params := url.Values{"_txlock": {"immediate"}, "modeof": {path}}
	u.RawQuery = params.Encode()
	s.db, err = sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, err
	}
	s.db.SetMaxOpenConns(1)
	s.db.SetMaxIdleConns(1)
	s.checkpoint = s.checkpointWAL
	if !fresh {
		var existingMode string
		if err = s.db.QueryRow("PRAGMA journal_mode").Scan(&existingMode); err != nil {
			return nil, err
		}
		if existingMode != "wal" {
			return nil, ErrInvalid
		}
		if err = s.validateDatabase(); err != nil {
			return nil, err
		}
	}
	// A single connection, no external readers (directory owner lock), no cache
	// spill, and a checkpoint after every bounded transaction constrain WAL size.
	for _, q := range []string{"PRAGMA trusted_schema=OFF", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=0", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA max_page_count=4096", "PRAGMA cache_size=-32768", "PRAGMA cache_spill=OFF", "PRAGMA wal_autocheckpoint=0", "PRAGMA journal_size_limit=0"} {
		if _, err = s.db.Exec(q); err != nil {
			return nil, err
		}
	}
	var mode string
	var syncMode, pageSize int
	if err = s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		return nil, err
	}
	if err = s.db.QueryRow("PRAGMA synchronous").Scan(&syncMode); err != nil {
		return nil, err
	}
	if err = s.db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return nil, err
	}
	if mode != "wal" || syncMode != 2 || pageSize != 4096 {
		return nil, ErrInvalid
	}
	if fresh {
		if _, err = s.db.Exec("BEGIN IMMEDIATE; " + schema + "; PRAGMA user_version=1; COMMIT"); err != nil {
			return nil, err
		}
	}
	if err = s.validateDatabase(); err != nil {
		return nil, err
	}
	recoveryNow := time.Now().UTC()
	if !validTime(recoveryNow) {
		return nil, ErrInvalid
	}
	if err = s.change(func(tx *sql.Tx) error {
		// Recovery time is at least the recorded start even if the host clock moved
		// backwards. No request is replayed and queued deadlines remain unchanged.
		_, e := tx.Exec("UPDATE jobs SET status='interrupted', finished_at=max(started_at,?) WHERE status='running'", recoveryNow.UnixNano())
		return e
	}); err != nil {
		return nil, err
	}
	if err = syncDirectory(dir); err != nil {
		return nil, err
	}
	return s, nil
}
func prepareDirectory(dir string) error {
	// Inspect each ancestor without resolving symlinks. Parent directory mutation
	// by the same uid/root is outside this private-state threat model.
	parts := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	cur := "/"
	for i, p := range parts {
		cur = filepath.Join(cur, p)
		fi, e := os.Lstat(cur)
		if os.IsNotExist(e) && i == len(parts)-1 {
			if e = os.Mkdir(cur, 0700); e != nil {
				return e
			}
			if e = syncDirectory(filepath.Dir(cur)); e != nil {
				return e
			}
			fi, e = os.Lstat(cur)
		}
		if e != nil {
			return e
		}
		if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			return ErrInvalid
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || (st.Uid != 0 && st.Uid != uint32(os.Geteuid())) {
			return ErrInvalid
		}
		if st.Mode&0022 != 0 && !(st.Uid == 0 && st.Mode&syscall.S_ISVTX != 0) {
			return ErrInvalid
		}
		if i == len(parts)-1 {
			if !ok || st.Uid != uint32(os.Geteuid()) || st.Mode&07777 != 0700 {
				return ErrInvalid
			}
		}
	}
	return nil
}
func privateFile(path string) (*os.File, error) {
	fd, e := syscall.Open(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	if e = checkFile(f); e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func checkFile(f *os.File) error {
	fi, e := f.Stat()
	if e != nil {
		return e
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || !fi.Mode().IsRegular() || st.Mode&07777 != 0600 || st.Uid != uint32(os.Geteuid()) || st.Nlink != 1 {
		return ErrInvalid
	}
	return nil
}
func (s *Store) checkFiles() error {
	entries, e := os.ReadDir(s.dir)
	if e != nil {
		return e
	}
	for _, v := range entries {
		name := v.Name()
		var max int64
		switch name {
		case "owner.lock":
			max = 0
		case databaseName:
			max = MaxDatabaseBytes
		case databaseName + "-wal":
			max = MaxWALBytes
		case databaseName + "-shm":
			max = 64 << 10
		default:
			return ErrInvalid
		}
		fd, e := syscall.Open(filepath.Join(s.dir, name), syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if e != nil {
			return e
		}
		f := os.NewFile(uintptr(fd), name)
		e = checkFile(f)
		fi, se := f.Stat()
		f.Close()
		if e != nil {
			return e
		}
		if se != nil {
			return se
		}
		if fi.Size() > max {
			return ErrFull
		}
	}
	return nil
}
func syncDirectory(dir string) error {
	f, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func (s *Store) validateDatabase() error {
	var version int
	var integrity string
	if e := s.db.QueryRow("PRAGMA user_version").Scan(&version); e != nil {
		return e
	}
	if version != 1 {
		return ErrInvalid
	}
	if e := s.db.QueryRow("PRAGMA integrity_check").Scan(&integrity); e != nil {
		return e
	}
	if integrity != "ok" {
		return ErrInvalid
	}
	rows, e := s.db.Query("SELECT type,name,coalesce(sql,'') FROM sqlite_master ORDER BY name")
	if e != nil {
		return e
	}
	count := 0
	for rows.Next() {
		var typ, name, ddl string
		if e = rows.Scan(&typ, &name, &ddl); e != nil {
			rows.Close()
			return e
		}
		switch name {
		case "jobs":
			if typ != "table" || ddl != schema {
				rows.Close()
				return ErrInvalid
			}
		case "sqlite_autoindex_jobs_1":
			if typ != "index" || ddl != "" {
				rows.Close()
				return ErrInvalid
			}
		default:
			rows.Close()
			return ErrInvalid
		}
		count++
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if count != 2 {
		return ErrInvalid
	}
	jobs, e := readJobs(s.db, "SELECT "+columns+" FROM jobs ORDER BY sequence LIMIT ?", MaxRetainedJobs+1)
	if e != nil {
		return e
	}
	if len(jobs) > MaxRetainedJobs {
		return ErrFull
	}
	active, queued := 0, 0
	for _, j := range jobs {
		if e = j.validate(); e != nil {
			return e
		}
		if j.Status == Running {
			active++
		}
		if j.Status == Queued {
			queued++
		}
	}
	if active > 1 || queued > MaxQueuedJobs {
		return ErrInvalid
	}
	return nil
}
func (s *Store) ready() error {
	if s.db == nil {
		return ErrClosed
	}
	if s.poisoned {
		return ErrPoisoned
	}
	if e := s.checkFiles(); e != nil {
		s.poisoned = true
		return e
	}
	return nil
}

// change poisons the store on any ambiguous storage failure. The caller must
// close/reopen and read persisted state before retrying a mutation.
func (s *Store) change(fn func(*sql.Tx) error) error {
	tx, e := s.db.Begin()
	if e != nil {
		s.poisoned = true
		return e
	}
	if e = fn(tx); e != nil {
		re := tx.Rollback()
		if re != nil {
			s.poisoned = true
		}
		if !errors.Is(e, ErrFull) && !errors.Is(e, ErrInvalid) && !errors.Is(e, ErrNotFound) && !errors.Is(e, ErrTransition) {
			s.poisoned = true
		}
		return e
	}
	if e = tx.Commit(); e != nil {
		s.poisoned = true
		return e
	}
	if e = s.checkpoint(); e != nil {
		s.poisoned = true
		return e
	}
	if e = s.checkFiles(); e != nil {
		s.poisoned = true
		return e
	}
	return nil
}
func (s *Store) checkpointWAL() error {
	var busy, log, done int
	if e := s.db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &done); e != nil {
		return e
	}
	if busy != 0 {
		return ErrPoisoned
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var e error
	if s.db != nil {
		e = s.db.Close()
		s.db = nil
	}
	if s.lock != nil {
		if le := s.lock.Close(); e == nil {
			e = le
		}
		s.lock = nil
	}
	return e
}

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
}

func readJobs(q queryer, query string, args ...any) ([]Job, error) {
	rows, e := q.Query(query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		var j Job
		var qt, rt, queued, started, finished, qd, rd int64
		if e = rows.Scan(&j.Sequence, &j.ID, &j.Request.Kind, &j.Request.CorrelationID, &j.Request.SessionID, &j.Request.MemoryMiB, &qt, &rt, &j.Status, &queued, &started, &finished, &qd, &rd); e != nil {
			return nil, e
		}
		j.Request.QueueTimeout = time.Duration(qt)
		j.Request.RunTimeout = time.Duration(rt)
		j.QueuedAt = stamp(queued)
		j.StartedAt = stamp(started)
		j.FinishedAt = stamp(finished)
		j.QueueDeadline = stamp(qd)
		j.RunDeadline = stamp(rd)
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
func stamp(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}
func nanos(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}
func get(q queryer, id string) (Job, error) {
	jobs, e := readJobs(q, "SELECT "+columns+" FROM jobs WHERE id=?", id)
	if e != nil {
		return Job{}, e
	}
	if len(jobs) != 1 {
		return Job{}, ErrNotFound
	}
	return jobs[0], nil
}
func (s *Store) Get(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(); e != nil {
		return Job{}, e
	}
	if !idPattern.MatchString(id) {
		return Job{}, ErrInvalid
	}
	return get(s.db, id)
}

// List returns at most limit jobs in durable FIFO sequence, including terminal
// history. Rows are never automatically deleted; a full store fails closed.
func (s *Store) List(limit int) ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(); e != nil {
		return nil, e
	}
	if limit <= 0 || limit > MaxRetainedJobs {
		return nil, ErrInvalid
	}
	return readJobs(s.db, "SELECT "+columns+" FROM jobs ORDER BY sequence LIMIT ?", limit)
}
func (s *Store) Enqueue(req Request, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(); e != nil {
		return Job{}, e
	}
	if req.validate() != nil || !validTime(now) || !validTime(now.Add(req.QueueTimeout)) {
		return Job{}, ErrInvalid
	}
	var random [16]byte
	if _, e := rand.Read(random[:]); e != nil {
		return Job{}, e
	}
	j := Job{ID: hex.EncodeToString(random[:]), Request: req, Status: Queued, QueuedAt: now.UTC(), QueueDeadline: now.Add(req.QueueTimeout).UTC()}
	e := s.change(func(tx *sql.Tx) error {
		var total, queued int
		if e := tx.QueryRow("SELECT count(*),coalesce(sum(status='queued'),0) FROM jobs").Scan(&total, &queued); e != nil {
			return e
		}
		if total >= MaxRetainedJobs || queued >= MaxQueuedJobs {
			return ErrFull
		}
		if e := tx.QueryRow("SELECT coalesce(max(sequence),0)+1 FROM jobs").Scan(&j.Sequence); e != nil {
			return e
		}
		_, e := tx.Exec("INSERT INTO jobs ("+columns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)", j.Sequence, j.ID, req.Kind, req.CorrelationID, req.SessionID, req.MemoryMiB, int64(req.QueueTimeout), int64(req.RunTimeout), j.Status, nanos(j.QueuedAt), 0, 0, nanos(j.QueueDeadline), 0)
		return e
	})
	if e != nil {
		return Job{}, e
	}
	return j, nil
}
func expire(tx *sql.Tx, now time.Time) error {
	_, e := tx.Exec("UPDATE jobs SET status='timed_out',finished_at=? WHERE (status='queued' AND queue_deadline<=?) OR (status='running' AND run_deadline<=?)", nanos(now), nanos(now), nanos(now))
	return e
}
func (s *Store) Expire(now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(); e != nil {
		return 0, e
	}
	if !validTime(now) {
		return 0, ErrInvalid
	}
	count := 0
	e := s.change(func(tx *sql.Tx) error {
		r, e := tx.Exec("UPDATE jobs SET status='timed_out',finished_at=? WHERE (status='queued' AND queue_deadline<=?) OR (status='running' AND run_deadline<=?)", nanos(now), nanos(now), nanos(now))
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		count = int(n)
		return e
	})
	if e != nil {
		return 0, e
	}
	return count, nil
}

// Admit expires elapsed jobs, then considers only the oldest queued job. An
// oversized head remains queued and blocks later jobs. Capacity is a cooperative
// metadata decision, not an execution or physical GPU allocation guarantee.
func (s *Store) Admit(snapshot Snapshot, policy Policy, now time.Time) (Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(); e != nil {
		return Job{}, false, e
	}
	if !validTime(now) || policy.validate() != nil {
		return Job{}, false, ErrInvalid
	}
	var admitted Job
	ok := false
	e := s.change(func(tx *sql.Tx) error {
		if e := expire(tx, now); e != nil {
			return e
		}
		var active int
		if e := tx.QueryRow("SELECT count(*) FROM jobs WHERE status='running'").Scan(&active); e != nil {
			return e
		}
		if active > 0 {
			return nil
		}
		if snapshot.TotalMiB <= 0 || snapshot.TotalMiB > MaxMemoryMiB || snapshot.FreeMiB < 0 || snapshot.FreeMiB > snapshot.TotalMiB || !validTime(snapshot.ObservedAt) || snapshot.ObservedAt.After(now) || now.Sub(snapshot.ObservedAt) > policy.MaxSnapshotAge {
			return nil
		}
		jobs, e := readJobs(tx, "SELECT "+columns+" FROM jobs WHERE status='queued' ORDER BY sequence LIMIT 1")
		if e != nil {
			return e
		}
		if len(jobs) == 0 {
			return nil
		}
		j := jobs[0]
		if now.Before(j.QueuedAt) {
			return ErrInvalid
		}
		cap := policy.BudgetMiB
		if snapshot.FreeMiB < cap {
			cap = snapshot.FreeMiB
		}
		if j.Request.MemoryMiB+policy.HeadroomMiB > cap {
			return nil
		}
		if !validTime(now.Add(j.Request.RunTimeout)) {
			return ErrInvalid
		}
		j.Status = Running
		j.StartedAt = now.UTC()
		j.RunDeadline = now.Add(j.Request.RunTimeout).UTC()
		_, e = tx.Exec("UPDATE jobs SET status='running',started_at=?,run_deadline=? WHERE id=?", nanos(j.StartedAt), nanos(j.RunDeadline), j.ID)
		if e == nil {
			admitted = j
			ok = true
		}
		return e
	})
	if e != nil {
		return Job{}, false, e
	}
	return admitted, ok, nil
}
func (s *Store) Finish(id string, status Status, now time.Time) (Job, error) {
	if status != Succeeded && status != Failed {
		return Job{}, ErrTransition
	}
	return s.terminal(id, status, now)
}
func (s *Store) Cancel(id string, now time.Time) (Job, error) { return s.terminal(id, Cancelled, now) }
func (s *Store) terminal(id string, status Status, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(); e != nil {
		return Job{}, e
	}
	if !validTime(now) || !idPattern.MatchString(id) {
		return Job{}, ErrInvalid
	}
	var result Job
	e := s.change(func(tx *sql.Tx) error {
		j, e := get(tx, id)
		if e != nil {
			return e
		}
		if j.Status != Running && (status != Cancelled || j.Status != Queued) {
			return ErrTransition
		}
		if now.Before(j.QueuedAt) || (!j.StartedAt.IsZero() && now.Before(j.StartedAt)) {
			return ErrInvalid
		}
		deadline := j.QueueDeadline
		if j.Status == Running {
			deadline = j.RunDeadline
		}
		target := status
		if !now.Before(deadline) {
			target = TimedOut
		}
		j.Status = target
		j.FinishedAt = now.UTC()
		_, e = tx.Exec("UPDATE jobs SET status=?,finished_at=? WHERE id=?", target, nanos(now), id)
		result = j
		return e
	})
	if e != nil {
		return Job{}, e
	}
	return result, nil
}
