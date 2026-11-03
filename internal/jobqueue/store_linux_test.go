//go:build linux

package jobqueue

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func request() Request {
	return Request{Kind: KindChat, CorrelationID: strings.Repeat("a", 32), SessionID: strings.Repeat("b", 32), MemoryMiB: 128, QueueTimeout: time.Minute, RunTimeout: time.Minute}
}
func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "queue")
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}
func enqueue(t *testing.T, s *Store) Job {
	t.Helper()
	j, e := s.Enqueue(request(), testNow)
	if e != nil {
		t.Fatal(e)
	}
	return j
}
func snapshot() Snapshot { return Snapshot{TotalMiB: 4096, FreeMiB: 2048, ObservedAt: testNow} }
func policy() Policy     { return Policy{BudgetMiB: 2048, HeadroomMiB: 128, MaxSnapshotAge: time.Minute} }

func TestSQLiteAndLimits(t *testing.T) {
	s, dir := testStore(t)
	var version string
	if e := s.db.QueryRow("SELECT sqlite_version()").Scan(&version); e != nil || version != "3.50.4" {
		t.Fatalf("SQLite %s: %v", version, e)
	}
	enqueue(t, s)
	for _, name := range []string{"owner.lock", databaseName, databaseName + "-wal", databaseName + "-shm"} {
		fi, e := os.Lstat(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		if fi.Mode().Perm() != 0600 {
			t.Fatalf("%s: %v", name, fi.Mode())
		}
	}
	for _, pragma := range []struct {
		name string
		want int
	}{{"synchronous", 2}, {"max_page_count", 4096}, {"cache_spill", 0}, {"wal_autocheckpoint", 0}} {
		var got int
		if e := s.db.QueryRow("PRAGMA " + pragma.name).Scan(&got); e != nil || got != pragma.want {
			t.Fatalf("%s=%d: %v", pragma.name, got, e)
		}
	}
}
func TestStrictMetadataAndClockBoundaries(t *testing.T) {
	s, _ := testStore(t)
	mutations := []func(*Request){func(r *Request) { r.Kind = "prompt:secret" }, func(r *Request) { r.SessionID = "Bearer token" }, func(r *Request) { r.CorrelationID = strings.Repeat("A", 32) }, func(r *Request) { r.MemoryMiB = 0 }, func(r *Request) { r.MemoryMiB = MaxMemoryMiB + 1 }, func(r *Request) { r.QueueTimeout = 24*time.Hour + 1 }, func(r *Request) { r.RunTimeout = time.Hour + 1 }, func(r *Request) { r.RunTimeout = 0 }}
	for i, mutate := range mutations {
		r := request()
		mutate(&r)
		if _, e := s.Enqueue(r, testNow); !errors.Is(e, ErrInvalid) {
			t.Fatalf("%d: %v", i, e)
		}
	}
	if _, e := s.Enqueue(request(), time.Unix(0, 0)); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	j := enqueue(t, s)
	if _, e := s.Finish(j.ID, Succeeded, testNow); !errors.Is(e, ErrTransition) {
		t.Fatal(e)
	}
	if _, e := s.Cancel(j.ID, testNow.Add(-time.Second)); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if n, e := s.Expire(testNow.Add(time.Minute - time.Nanosecond)); e != nil || n != 0 {
		t.Fatalf("%d %v", n, e)
	}
	if n, e := s.Expire(testNow.Add(time.Minute)); e != nil || n != 1 {
		t.Fatalf("%d %v", n, e)
	}
	if _, e := s.Cancel(j.ID, testNow.Add(time.Minute)); !errors.Is(e, ErrTransition) {
		t.Fatal(e)
	}
}
func TestAdmissionFIFOAndFreshness(t *testing.T) {
	s, _ := testStore(t)
	big := request()
	big.MemoryMiB = 2000
	head, e := s.Enqueue(big, testNow)
	if e != nil {
		t.Fatal(e)
	}
	next := enqueue(t, s)
	p := policy()
	for _, snap := range []Snapshot{{}, {TotalMiB: 4000, FreeMiB: 5000, ObservedAt: testNow}, {TotalMiB: 4000, FreeMiB: 4000, ObservedAt: testNow.Add(1)}, {TotalMiB: 4000, FreeMiB: 4000, ObservedAt: testNow.Add(-time.Minute - 1)}} {
		if j, ok, e := s.Admit(snap, p, testNow); e != nil || ok || j.ID != "" {
			t.Fatalf("%+v: %v %v", snap, ok, e)
		}
	}
	if _, ok, e := s.Admit(snapshot(), p, testNow); e != nil || ok {
		t.Fatalf("head should block: %v %v", ok, e)
	}
	if j, e := s.Get(next.ID); e != nil || j.Status != Queued {
		t.Fatal(j, e)
	}
	if _, e = s.Cancel(head.ID, testNow); e != nil {
		t.Fatal(e)
	}
	j, ok, e := s.Admit(snapshot(), p, testNow)
	if e != nil || !ok || j.ID != next.ID {
		t.Fatal(j, ok, e)
	}
	if _, ok, e = s.Admit(snapshot(), p, testNow); e != nil || ok {
		t.Fatal(ok, e)
	}
	if _, e = s.Finish(j.ID, Succeeded, testNow.Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	j, e = s.Get(j.ID)
	if e != nil || j.Status != TimedOut {
		t.Fatal(j, e)
	}
}
func TestRestartAndOwnership(t *testing.T) {
	s, dir := testStore(t)
	j := enqueue(t, s)
	other := enqueue(t, s)
	if _, ok, e := s.Admit(snapshot(), policy(), testNow); e != nil || !ok {
		t.Fatal(ok, e)
	}
	if _, e := Open(dir); !errors.Is(e, ErrOwned) {
		t.Fatal(e)
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	got, e := s.Get(j.ID)
	if e != nil || got.Status != Interrupted {
		t.Fatal(got, e)
	}
	got, e = s.Get(other.ID)
	if e != nil || got.Status != Queued {
		t.Fatal(got, e)
	}
}
func TestConcurrentSingleActive(t *testing.T) {
	s, _ := testStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Enqueue(request(), testNow); errs <- e }()
	}
	wg.Wait()
	for i := 0; i < 32; i++ {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
	}
	admitted := make(chan bool, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, ok, e := s.Admit(snapshot(), policy(), testNow); errs <- e; admitted <- ok }()
	}
	wg.Wait()
	total := 0
	for i := 0; i < 32; i++ {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
		if <-admitted {
			total++
		}
	}
	if total != 1 {
		t.Fatal(total)
	}
}
func TestQueueProcessHelper(t *testing.T) {
	mode := os.Getenv("LOCAS_QUEUE_TEST_MODE")
	if mode == "" {
		return
	}
	s, e := Open(os.Getenv("LOCAS_QUEUE_TEST_DIR"))
	if mode == "owned" {
		if !errors.Is(e, ErrOwned) {
			os.Exit(21)
		}
		os.Exit(0)
	}
	if e != nil {
		os.Exit(22)
	}
	if _, e = s.Enqueue(request(), testNow); e != nil {
		os.Exit(23)
	}
	if _, ok, e := s.Admit(snapshot(), policy(), testNow); e != nil || !ok {
		os.Exit(24)
	}
	if _, e = s.Enqueue(request(), testNow); e != nil {
		os.Exit(25)
	}
	os.Exit(0)
}
func TestProcessOwnershipAndAbruptRestart(t *testing.T) {
	s, dir := testStore(t)
	run := func(mode string) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestQueueProcessHelper$")
		cmd.Env = append(os.Environ(), "LOCAS_QUEUE_TEST_MODE="+mode, "LOCAS_QUEUE_TEST_DIR="+dir)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("%s: %v %s", mode, e, out)
		}
	}
	run("owned")
	s.Close()
	run("crash")
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	jobs, e := s.List(10)
	if e != nil || len(jobs) != 2 || jobs[0].Status != Interrupted || jobs[1].Status != Queued {
		t.Fatal(jobs, e)
	}
}
func TestUnsafePaths(t *testing.T) {
	for _, name := range []string{"owner.lock", databaseName, databaseName + "-wal", databaseName + "-shm"} {
		for _, kind := range []string{"symlink", "hardlink", "fifo", "mode", "special"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "queue")
				if e := os.Mkdir(dir, 0700); e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(dir, name)
				target := filepath.Join(t.TempDir(), "target")
				if e := os.WriteFile(target, nil, 0600); e != nil {
					t.Fatal(e)
				}
				var e error
				switch kind {
				case "symlink":
					e = os.Symlink(target, path)
				case "hardlink":
					e = os.Link(target, path)
				case "fifo":
					e = syscall.Mkfifo(path, 0600)
				case "mode":
					e = os.WriteFile(path, nil, 0600)
					if e == nil {
						e = os.Chmod(path, 0644)
					}
				case "special":
					e = os.WriteFile(path, nil, 0600)
					if e == nil {
						e = syscall.Chmod(path, 04600)
					}
				}
				if e != nil {
					t.Fatal(e)
				}
				if s, e := Open(dir); e == nil {
					s.Close()
					t.Fatal("unsafe file accepted")
				}
			})
		}
	}
	t.Run("ancestor-symlink", func(t *testing.T) {
		base := t.TempDir()
		target := filepath.Join(base, "target")
		os.Mkdir(target, 0700)
		link := filepath.Join(base, "link")
		os.Symlink(target, link)
		if s, e := Open(filepath.Join(link, "queue")); e == nil {
			s.Close()
			t.Fatal("accepted")
		}
	})
	t.Run("writable-ancestor", func(t *testing.T) {
		base := t.TempDir()
		os.Chmod(base, 0777)
		if s, e := Open(filepath.Join(base, "queue")); e == nil {
			s.Close()
			t.Fatal("accepted")
		}
	})
}
func TestSchemaAndCorruptionFailClosed(t *testing.T) {
	for _, mutation := range []string{"PRAGMA user_version=2", "CREATE VIEW surprise AS SELECT * FROM jobs", "UPDATE jobs SET status='secret error'", "UPDATE jobs SET memory_mib=0", "UPDATE jobs SET started_at=1", "UPDATE jobs SET correlation_id='secret'"} {
		t.Run(mutation, func(t *testing.T) {
			s, dir := testStore(t)
			enqueue(t, s)
			if _, e := s.db.Exec(mutation); e != nil {
				t.Fatal(e)
			}
			s.Close()
			if reopened, e := Open(dir); e == nil {
				reopened.Close()
				t.Fatal("invalid DB accepted")
			}
		})
	}
	t.Run("zero-file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "queue")
		os.Mkdir(dir, 0700)
		os.WriteFile(filepath.Join(dir, databaseName), nil, 0600)
		if s, e := Open(dir); e == nil {
			s.Close()
			t.Fatal("empty existing DB accepted")
		}
	})
	t.Run("corrupt-header", func(t *testing.T) {
		s, dir := testStore(t)
		s.Close()
		f, e := os.OpenFile(filepath.Join(dir, databaseName), os.O_WRONLY, 0)
		if e != nil {
			t.Fatal(e)
		}
		_, e = f.WriteAt([]byte("not sqlite"), 0)
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		if s, e := Open(dir); e == nil {
			s.Close()
			t.Fatal("corrupt DB accepted")
		}
	})
}
func TestRowAndVolumeBounds(t *testing.T) {
	for _, queued := range []bool{true, false} {
		t.Run(fmt.Sprint(queued), func(t *testing.T) {
			s, dir := testStore(t)
			j := enqueue(t, s)
			limit := MaxRetainedJobs
			status := Cancelled
			if queued {
				limit = MaxQueuedJobs
				status = Queued
			}
			if !queued {
				if _, ok, e := s.Admit(snapshot(), policy(), testNow); e != nil || !ok {
					t.Fatal("full-store setup", e)
				}
			}
			e := s.change(func(tx *sql.Tx) error {
				for i := 2; i <= limit; i++ {
					_, e := tx.Exec("INSERT INTO jobs ("+columns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)", i, fmt.Sprintf("%032x", i), j.Request.Kind, j.Request.CorrelationID, j.Request.SessionID, j.Request.MemoryMiB, int64(j.Request.QueueTimeout), int64(j.Request.RunTimeout), status, nanos(testNow), 0, map[bool]int64{true: 0, false: nanos(testNow)}[queued], nanos(j.QueueDeadline), 0)
					if e != nil {
						return e
					}
				}
				return nil
			})
			if e != nil {
				t.Fatal(e)
			}
			if _, e := s.Enqueue(request(), testNow); !errors.Is(e, ErrFull) {
				t.Fatal(e)
			}
			if queued {
				if _, e := s.Cancel(j.ID, testNow); e != nil {
					t.Fatal(e)
				}
				if _, e := s.Enqueue(request(), testNow); e != nil {
					t.Fatal(e)
				}
			} else {
				if done, e := s.Finish(j.ID, Succeeded, testNow.Add(time.Second)); e != nil || done.Status != Succeeded {
					t.Fatal("terminal transition at retained limit", e)
				}
				if _, e := s.Enqueue(request(), testNow); !errors.Is(e, ErrFull) {
					t.Fatal("terminal history silently evicted", e)
				}
			}
			if e = s.checkFiles(); e != nil {
				t.Fatal(e)
			}
			fi, e := os.Stat(filepath.Join(dir, databaseName))
			if e != nil || fi.Size() > MaxDatabaseBytes/2 {
				t.Fatal(fi, e)
			}
			s.Close()
			s, e = Open(dir)
			if e != nil {
				t.Fatal(e)
			}
			s.Close()
		})
	}
	t.Run("oversized-db", func(t *testing.T) {
		s, dir := testStore(t)
		s.Close()
		if e := os.Truncate(filepath.Join(dir, databaseName), MaxDatabaseBytes+1); e != nil {
			t.Fatal(e)
		}
		if s, e := Open(dir); e == nil {
			s.Close()
			t.Fatal("oversized accepted")
		}
	})
}
func TestStorageFailurePoisons(t *testing.T) {
	s, dir := testStore(t)
	if e := os.Chmod(filepath.Join(dir, databaseName), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Enqueue(request(), testNow); e == nil {
		t.Fatal("unsafe file accepted")
	}
	os.Chmod(filepath.Join(dir, databaseName), 0600)
	if _, e := s.List(1); !errors.Is(e, ErrPoisoned) {
		t.Fatal(e)
	}
}

func TestSQLiteWriteFailureRequiresReopen(t *testing.T) {
	s, dir := testStore(t)
	if _, err := s.db.Exec("PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	if job, err := s.Enqueue(request(), testNow); err == nil || job.ID != "" {
		t.Fatalf("write failure reported success %+v %v", job, err)
	}
	if _, err := s.db.Exec("PRAGMA query_only=OFF"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(1); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("failed writer resumed without reconciliation: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	jobs, err := reopened.List(10)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("failed insert persisted %+v %v", jobs, err)
	}
	if _, err := reopened.Enqueue(request(), testNow); err != nil {
		t.Fatal(err)
	}
}

func TestPostCommitFailureNeverReturnsNormalSuccess(t *testing.T) {
	for _, operation := range []string{"enqueue", "admit", "expire", "finish", "cancel"} {
		t.Run(operation, func(t *testing.T) {
			s, dir := testStore(t)
			job := enqueue(t, s)
			if operation == "finish" {
				if _, ok, err := s.Admit(snapshot(), policy(), testNow); err != nil || !ok {
					t.Fatal("setup admission", err)
				}
			}
			fault := errors.New("injected checkpoint failure after real commit")
			s.checkpoint = func() error { return fault }
			var err error
			switch operation {
			case "enqueue":
				var result Job
				result, err = s.Enqueue(request(), testNow)
				if result.ID != "" {
					t.Fatal("uncertain enqueue returned job")
				}
			case "admit":
				var result Job
				var ok bool
				result, ok, err = s.Admit(snapshot(), policy(), testNow)
				if ok || result.ID != "" {
					t.Fatal("uncertain admission released a reservation")
				}
			case "expire":
				var count int
				count, err = s.Expire(testNow.Add(time.Minute))
				if count != 0 {
					t.Fatal("uncertain expiry returned normal count")
				}
			case "finish":
				var result Job
				result, err = s.Finish(job.ID, Succeeded, testNow.Add(time.Second))
				if result.ID != "" {
					t.Fatal("uncertain completion returned job")
				}
			case "cancel":
				var result Job
				result, err = s.Cancel(job.ID, testNow)
				if result.ID != "" {
					t.Fatal("uncertain cancellation returned job")
				}
			}
			if !errors.Is(err, fault) {
				t.Fatalf("missing storage fault %v", err)
			}
			if _, err := s.Get(job.ID); !errors.Is(err, ErrPoisoned) {
				t.Fatalf("uncertain writer resumed: %v", err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			rows, err := reopened.List(10)
			if err != nil {
				t.Fatal(err)
			}
			expected := map[string]Status{"enqueue": Queued, "admit": Interrupted, "expire": TimedOut, "finish": Succeeded, "cancel": Cancelled}[operation]
			if len(rows) == 0 || rows[0].Status != expected {
				t.Fatalf("committed state not reconciled %+v", rows)
			}
			if operation == "enqueue" && len(rows) != 2 {
				t.Fatal("ambiguous enqueue commit lost")
			}
		})
	}
}

func TestDeadlineRangeIsValidatedOnRecovery(t *testing.T) {
	queued := time.Date(2200, 12, 31, 23, 59, 0, 0, time.UTC)
	req := request()
	req.QueueTimeout = 2 * time.Minute
	job := Job{ID: strings.Repeat("c", 32), Sequence: 1, Request: req, Status: Queued, QueuedAt: queued, QueueDeadline: queued.Add(req.QueueTimeout)}
	if job.validate() == nil {
		t.Fatal("out-of-range queued deadline accepted")
	}
	req.QueueTimeout = time.Minute
	job = Job{ID: strings.Repeat("c", 32), Sequence: 1, Request: req, Status: Running, QueuedAt: queued.Add(-time.Minute), QueueDeadline: queued, StartedAt: queued.Add(-time.Second), RunDeadline: queued.Add(-time.Second).Add(req.RunTimeout)}
	job.Request.RunTimeout = 2 * time.Minute
	job.RunDeadline = job.StartedAt.Add(job.Request.RunTimeout)
	if job.validate() == nil {
		t.Fatal("out-of-range run deadline accepted")
	}
}
