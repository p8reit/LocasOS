//go:build linux

package jobqueue_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/p8reit/LocasOS/internal/jobqueue"
)

func request() jobqueue.Request {
	return jobqueue.Request{Kind: jobqueue.KindChat, CorrelationID: strings.Repeat("a", 32), SessionID: strings.Repeat("b", 32), MemoryMiB: 4096, QueueTimeout: time.Hour, RunTimeout: time.Minute}
}
func fixture(t *testing.T) (*jobqueue.Store, string, time.Time) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private-queue")
	s, err := jobqueue.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir, time.Date(2026, 11, 3, 12, 0, 0, 123456789, time.UTC)
}
func capacity(now time.Time) (jobqueue.Snapshot, jobqueue.Policy) {
	return jobqueue.Snapshot{TotalMiB: 8192, FreeMiB: 8192, ObservedAt: now}, jobqueue.Policy{BudgetMiB: 8192, HeadroomMiB: 512, MaxSnapshotAge: 5 * time.Second}
}
func TestDurableFIFOAdmissionRecoveryAndTerminalProtection(t *testing.T) {
	s, dir, now := fixture(t)
	first, err := s.Enqueue(request(), now)
	if err != nil {
		t.Fatal(err)
	}
	smaller := request()
	smaller.MemoryMiB = 1024
	second, err := s.Enqueue(smaller, now)
	if err != nil {
		t.Fatal(err)
	}
	snap, policy := capacity(now)
	snap.FreeMiB = 2048
	if _, ok, err := s.Admit(snap, policy, now); err != nil || ok {
		t.Fatalf("FIFO head bypassed by smaller request: %v %v", ok, err)
	}
	snap.FreeMiB = 8192
	running, ok, err := s.Admit(snap, policy, now)
	if err != nil || !ok || running.ID != first.ID {
		t.Fatalf("first admission %+v %v %v", running, ok, err)
	}
	if _, ok, err := s.Admit(snap, policy, now); err != nil || ok {
		t.Fatal("two active reservations")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := jobqueue.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	interrupted, err := reopened.Get(first.ID)
	if err != nil || interrupted.Status != jobqueue.Interrupted {
		t.Fatalf("inflight recovery %+v %v", interrupted, err)
	}
	queued, err := reopened.Get(second.ID)
	if err != nil || queued.Status != jobqueue.Queued {
		t.Fatalf("queued work lost %+v %v", queued, err)
	}
	if _, err := reopened.Finish(first.ID, jobqueue.Succeeded, now.Add(time.Second)); !errors.Is(err, jobqueue.ErrTransition) {
		t.Fatalf("stale completion accepted %v", err)
	}
	// Metadata admission is explicitly not proof that a restarted GPU is quiescent.
	next, ok, err := reopened.Admit(snap, policy, now.Add(time.Second))
	if err != nil || !ok || next.ID != second.ID {
		t.Fatalf("queued metadata unavailable %+v %v %v", next, ok, err)
	}
	done, err := reopened.Finish(second.ID, jobqueue.Succeeded, now.Add(2*time.Second))
	if err != nil || done.Status != jobqueue.Succeeded {
		t.Fatalf("finish %+v %v", done, err)
	}
	if _, err := reopened.Cancel(second.ID, now.Add(3*time.Second)); !errors.Is(err, jobqueue.ErrTransition) {
		t.Fatal("terminal job changed")
	}
}
func TestConcurrentAdmissionHasOneWinner(t *testing.T) {
	s, _, now := fixture(t)
	for i := 0; i < 20; i++ {
		if _, err := s.Enqueue(request(), now); err != nil {
			t.Fatal(err)
		}
	}
	snap, policy := capacity(now)
	var wg sync.WaitGroup
	results := make(chan bool, 32)
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, ok, err := s.Admit(snap, policy, now); results <- ok; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	winners := 0
	for ok := range results {
		if ok {
			winners++
		}
	}
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if winners != 1 {
		t.Fatalf("admission winners %d", winners)
	}
}
func TestExactTimeoutAndCancelledQueueCannotDispatch(t *testing.T) {
	s, _, now := fixture(t)
	req := request()
	req.QueueTimeout = time.Second
	queued, err := s.Enqueue(req, now)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.Expire(now.Add(time.Second)); err != nil || n != 1 {
		t.Fatalf("expiry %d %v", n, err)
	}
	timed, err := s.Get(queued.ID)
	if err != nil || timed.Status != jobqueue.TimedOut {
		t.Fatalf("timeout %+v %v", timed, err)
	}
	job, err := s.Enqueue(request(), now)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := s.Cancel(job.ID, now.Add(time.Second))
	if err != nil || cancelled.Status != jobqueue.Cancelled {
		t.Fatal("cancel", err)
	}
	snap, policy := capacity(now)
	if _, ok, err := s.Admit(snap, policy, now.Add(time.Second)); err != nil || ok {
		t.Fatalf("terminal admission %v %v", ok, err)
	}
}
func TestOtherProcessCannotTakeOwnership(t *testing.T) {
	if dir := os.Getenv("LOCAS_QUEUE_TEST_CHILD_DIR"); dir != "" {
		s, err := jobqueue.Open(dir)
		if s != nil {
			_ = s.Close()
		}
		if !errors.Is(err, jobqueue.ErrOwned) {
			t.Fatalf("foreign owner acquired %v", err)
		}
		return
	}
	_, dir, _ := fixture(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestOtherProcessCannotTakeOwnership$")
	cmd.Env = append(os.Environ(), "LOCAS_QUEUE_TEST_CHILD_DIR="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ownership child %v: %s", err, output)
	}
}

func TestAbruptOwnerDeathPreservesCommittedMetadata(t *testing.T) {
	if dir := os.Getenv("LOCAS_QUEUE_CRASH_CHILD_DIR"); dir != "" {
		s, err := jobqueue.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		first, err := s.Enqueue(request(), now)
		if err != nil {
			t.Fatal(err)
		}
		second, err := s.Enqueue(request(), now)
		if err != nil {
			t.Fatal(err)
		}
		snap, policy := capacity(now)
		if _, ok, err := s.Admit(snap, policy, now); err != nil || !ok {
			t.Fatal("child admission", err)
		}
		data, _ := json.Marshal([]string{first.ID, second.ID})
		if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "receipts.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		// Kill only this disposable test child, bypassing all store/SQLite cleanup.
		if err := syscall.Kill(os.Getpid(), syscall.SIGKILL); err != nil {
			t.Fatal(err)
		}
		t.Fatal("child kill returned")
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "queue")
	cmd := exec.Command(os.Args[0], "-test.run=^TestAbruptOwnerDeathPreservesCommittedMetadata$")
	cmd.Env = append(os.Environ(), "LOCAS_QUEUE_CRASH_CHILD_DIR="+dir)
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		t.Fatalf("child did not reach crash checkpoint: %v %s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(parent, "receipts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil || len(ids) != 2 {
		t.Fatal("missing receipts", err)
	}
	s, err := jobqueue.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i, want := range []jobqueue.Status{jobqueue.Interrupted, jobqueue.Queued} {
		job, err := s.Get(ids[i])
		if err != nil || job.Status != want {
			t.Fatalf("post-crash job %+v %v", job, err)
		}
	}
}
func TestMissingDatabaseDoesNotBecomeEmptyHistory(t *testing.T) {
	s, dir, now := fixture(t)
	if _, err := s.Enqueue(request(), now); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "queue.db")); err != nil {
		t.Fatal(err)
	}
	reopened, err := jobqueue.Open(dir)
	if reopened != nil {
		defer reopened.Close()
	}
	if err == nil {
		t.Fatal("lost database silently initialized")
	}
	if _, err := os.Stat(filepath.Join(dir, "queue.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing database was recreated")
	}
}
