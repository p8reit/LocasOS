package auditlog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func event(n int) Event {
	return Event{CorrelationID: fmt.Sprintf("%032x", n), Service: "core", Operation: "model.pull", Stage: "intent", Principal: "administrator", Source: "browser", Outcome: "pending"}
}
func setup(t *testing.T) (string, *Journal) {
	t.Helper()
	d := filepath.Join(t.TempDir(), "audit")
	j, e := Open(d, "core")
	if e != nil {
		t.Fatal(e)
	}
	return d, j
}
func complete(t *testing.T, j *Journal, n int) {
	t.Helper()
	e := event(n)
	if err := j.Append(e); err != nil {
		t.Fatal(err)
	}
	e.Stage = "result"
	e.Outcome = "succeeded"
	if err := j.Append(e); err != nil {
		t.Fatal(err)
	}
}

func TestRestartAndVerification(t *testing.T) {
	d, j := setup(t)
	complete(t, j, 1)
	e := event(2)
	if err := j.Append(e); err != nil {
		t.Fatal(err)
	}
	j, err := Open(d, "core")
	if err != nil {
		t.Fatal(err)
	}
	e.Stage = "result"
	e.Outcome = "uncertain"
	if err = j.Append(e); err != nil {
		t.Fatal(err)
	}
	if err = j.Append(event(3)); err != nil {
		t.Fatal(err)
	}
	r, err := Verify(d)
	if err != nil || r.Entries != 5 || len(r.LastHash) != 64 || len(r.Pending) != 1 || len(r.Uncertain) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err = Open(d, "agent"); err == nil {
		t.Fatal("mixed service accepted")
	}
	if err = j.Append(event(1)); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestConcurrentAndProcessAppend(t *testing.T) {
	if os.Getenv("AUDIT_TEST_CHILD") == "1" {
		j, e := Open(os.Getenv("AUDIT_TEST_DIR"), "core")
		if e != nil {
			t.Fatal(e)
		}
		complete(t, j, 900)
		return
	}
	d, j := setup(t)
	var wg sync.WaitGroup
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			other, e := Open(d, "core")
			if e != nil {
				t.Error(e)
				return
			}
			complete(t, other, n)
		}(i)
	}
	c := exec.Command(os.Args[0], "-test.run=^TestConcurrentAndProcessAppend$")
	c.Env = append(os.Environ(), "AUDIT_TEST_CHILD=1", "AUDIT_TEST_DIR="+d)
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("%v %s", e, out)
	}
	wg.Wait()
	complete(t, j, 999)
	r, e := Verify(d)
	if e != nil || r.Entries != 44 {
		t.Fatalf("%+v %v", r, e)
	}
}

func TestTamperAndPartial(t *testing.T) {
	for _, kind := range []string{"tamper", "partial", "unknown", "oversize", "duplicate-json"} {
		t.Run(kind, func(t *testing.T) {
			d, j := setup(t)
			complete(t, j, 1)
			p := filepath.Join(d, journalName)
			b, _ := os.ReadFile(p)
			switch kind {
			case "tamper":
				b = bytes.Replace(b, []byte("administrator"), []byte("anonymous"), 1)
			case "partial":
				b = b[:len(b)-1]
			case "unknown":
				b = bytes.Replace(b, []byte(`"event":`), []byte(`"secret":"password","event":`), 1)
			case "oversize":
				b = append(b, []byte(strings.Repeat("x", maxLine+1))...)
			case "duplicate-json":
				b = bytes.Replace(b, []byte(`"writer_uid":`), []byte(`"writer_uid":0,"writer_uid":`), 1)
			}
			if e := os.WriteFile(p, b, 0600); e != nil {
				t.Fatal(e)
			}
			if _, e := Verify(d); e == nil {
				t.Fatal("corruption accepted")
			}
			if _, e := Open(d, "core"); e == nil {
				t.Fatal("corrupt restart accepted")
			}
			if e := j.Append(event(2)); e == nil {
				t.Fatal("corrupt append accepted")
			}
		})
	}
}

func TestInvalidFieldsDoNotPersist(t *testing.T) {
	d, j := setup(t)
	changes := []func(*Event){func(e *Event) { e.CorrelationID = "secret" }, func(e *Event) { e.SessionID = "cookie" }, func(e *Event) { e.Operation = "curl https://secret" }, func(e *Event) { e.Service = "evil" }, func(e *Event) { e.Source = "header" }, func(e *Event) { e.Outcome = "password" }, func(e *Event) { e.Principal = "root" }, func(e *Event) { e.Stage = "password" }, func(e *Event) { e.App = "/private" }, func(e *Event) { e.Version = "v1.2.3; secret" }, func(e *Event) { e.Channel = "private" }, func(e *Event) { e.SHA256 = strings.Repeat("A", 64) }, func(e *Event) { e.HTTPStatus = 99 }}
	for _, change := range changes {
		e := event(1)
		change(&e)
		if err := j.Append(e); err == nil {
			t.Fatalf("accepted %+v", e)
		}
	}
	b, _ := os.ReadFile(filepath.Join(d, journalName))
	if len(b) != 0 {
		t.Fatal("invalid data persisted")
	}
}

func TestBudgetReservesResults(t *testing.T) {
	d, j := setup(t)
	j.capacity = MaxEntryBytes + 800
	if e := j.Append(event(1)); e != nil {
		t.Fatal(e)
	}
	if e := j.Append(event(2)); e == nil {
		t.Fatal("reservation overflow accepted")
	}
	st, _ := os.Stat(filepath.Join(d, journalName))
	j.capacity = st.Size() + MaxEntryBytes
	inter := event(1)
	inter.Stage = "verified"
	if e := j.Append(inter); e == nil {
		t.Fatal("intermediate consumed reserved result space")
	}
	result := event(1)
	result.Stage = "result"
	result.Outcome = "succeeded"
	if e := j.Append(result); e != nil {
		t.Fatal(e)
	}
	if r, e := Verify(d); e != nil || r.Entries != 2 || len(r.Pending) != 0 {
		t.Fatalf("%+v %v", r, e)
	}
}

func TestPersistenceFailurePoisons(t *testing.T) {
	for _, kind := range []string{"partial", "sync"} {
		t.Run(kind, func(t *testing.T) {
			d, j := setup(t)
			if kind == "partial" {
				j.write = func(f *os.File, b []byte) (int, error) { n, _ := f.Write(b[:10]); return n, errors.New("injected") }
			} else {
				j.syncFile = func(*os.File) error { return errors.New("injected") }
			}
			if e := j.Append(event(1)); e == nil {
				t.Fatal("failure ignored")
			}
			if e := j.Append(event(2)); e == nil {
				t.Fatal("poison ignored")
			}
			r, e := Verify(d)
			if kind == "partial" && e == nil {
				t.Fatal("partial accepted")
			}
			if kind == "sync" && (e != nil || len(r.Pending) != 1) {
				t.Fatalf("uncertain durable intent missing: %+v %v", r, e)
			}
		})
	}
}

func TestUnsafePaths(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo", "hardlink", "file-mode", "dir-mode", "directory", "ancestor-symlink"} {
		t.Run(kind, func(t *testing.T) {
			d, _ := setup(t)
			p := filepath.Join(d, journalName)
			switch kind {
			case "file-mode":
				os.Chmod(p, 0640)
			case "dir-mode":
				os.Chmod(d, 0750)
			case "hardlink":
				if e := os.Link(p, p+".link"); e != nil {
					t.Fatal(e)
				}
			case "ancestor-symlink":
				link := filepath.Join(filepath.Dir(d), "link")
				os.Symlink(d, link)
				d = link
			default:
				os.Remove(p)
				switch kind {
				case "symlink":
					os.Symlink(filepath.Join(t.TempDir(), "target"), p)
				case "fifo":
					syscall.Mkfifo(p, 0600)
				case "directory":
					os.Mkdir(p, 0700)
				}
			}
			if _, e := Open(d, "core"); e == nil {
				t.Fatal("unsafe path accepted")
			}
			if _, e := Verify(d); e == nil {
				t.Fatal("unsafe verify accepted")
			}
		})
	}
}

func TestCreationAndReadOnly(t *testing.T) {
	d := filepath.Join(t.TempDir(), "absent")
	if _, e := Verify(d); e == nil {
		t.Fatal("created absent journal")
	}
	if _, e := os.Stat(d); !os.IsNotExist(e) {
		t.Fatal("verify created state")
	}
	p := filepath.Join(t.TempDir(), "file")
	os.WriteFile(p, []byte("x"), 0600)
	if _, e := Open(filepath.Join(p, "audit"), "core"); e == nil {
		t.Fatal("invalid parent accepted")
	}
	d, j := setup(t)
	os.Remove(filepath.Join(d, journalName))
	if e := j.Append(event(1)); e == nil {
		t.Fatal("append recreated deleted journal")
	}
}

func TestConcurrentReservationsAndLargeResult(t *testing.T) {
	d, j := setup(t)
	other, err := Open(d, "core")
	if err != nil {
		t.Fatal(err)
	}
	j.capacity = MaxEntryBytes + 1000
	other.capacity = j.capacity
	start := make(chan struct{})
	results := make(chan int, 2)
	for i, writer := range []*Journal{j, other} {
		go func(n int, w *Journal) {
			<-start
			if e := w.Append(event(n)); e == nil {
				results <- n
			} else {
				results <- 0
			}
		}(i+1, writer)
	}
	close(start)
	a, b := <-results, <-results
	if (a == 0) == (b == 0) {
		t.Fatalf("expected exactly one reserved intent, got %d %d", a, b)
	}
	n := a + b
	e := event(n)
	e.Stage = "result"
	e.Outcome = "succeeded"
	e.SessionID = strings.Repeat("a", 32)
	e.SHA256 = strings.Repeat("b", 64)
	e.Version = "v123456789.123456789.123456789"
	e.App = "open-webui"
	e.Channel = "stable"
	e.HTTPStatus = 599
	uid := uint32(os.Geteuid())
	e.PeerUID = &uid
	if err := j.Append(e); err != nil {
		t.Fatal(err)
	}
	if r, err := Verify(d); err != nil || len(r.Pending) != 0 || r.Entries != 2 {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestResultRequiresIntentAndMultipleOperationPending(t *testing.T) {
	d, j := setup(t)
	e := event(1)
	e.Stage = "result"
	e.Outcome = "denied"
	if err := j.Append(e); err == nil {
		t.Fatal("orphan outcome accepted")
	}
	e = event(1)
	if err := j.Append(e); err != nil {
		t.Fatal(err)
	}
	e.Operation = "model.chat"
	if err := j.Append(e); err != nil {
		t.Fatal(err)
	}
	e.Stage = "result"
	e.Outcome = "succeeded"
	if err := j.Append(e); err != nil {
		t.Fatal(err)
	}
	r, err := Verify(d)
	if err != nil || len(r.Pending) != 1 {
		t.Fatalf("other pending operation lost: %+v %v", r, err)
	}
}
