package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/p8reit/LocasOS/internal/auditlog"
)

func cliJournal(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "audit")
	j, err := auditlog.Open(d, "core")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		e := auditlog.Event{CorrelationID: strings.Repeat(string(rune('a'+i)), 32), Service: "core", Operation: "model.pull", Stage: "intent", Principal: "administrator", Source: "browser", Outcome: "pending"}
		if err := j.Append(e); err != nil {
			t.Fatal(err)
		}
		if i != 1 {
			e.Stage = "result"
			e.Outcome = "succeeded"
			if i == 2 {
				e.Outcome = "uncertain"
			}
			if err := j.Append(e); err != nil {
				t.Fatal(err)
			}
		}
	}
	return d
}

func snapshot(t *testing.T, d string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}
	s := map[string][]byte{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(d, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		s[e.Name()] = b
	}
	return s
}

func TestVerifyReadOnlyReportsOutstandingOperations(t *testing.T) {
	d := cliJournal(t)
	before := snapshot(t, d)
	var out, stderr bytes.Buffer
	if code := run([]string{"verify", d}, &out, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatal(stderr.String())
	}
	var r auditlog.Report
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Service != "core" || r.Entries != 5 || len(r.LastHash) != 64 || len(r.Pending) != 1 || r.Pending[0] != strings.Repeat("b", 32) || len(r.Uncertain) != 1 || r.Uncertain[0] != strings.Repeat("c", 32) {
		t.Fatalf("%+v", r)
	}
	after := snapshot(t, d)
	if len(before) != len(after) {
		t.Fatal("verification created files")
	}
	for p, b := range before {
		if !bytes.Equal(b, after[p]) {
			t.Fatal("verification modified journal")
		}
	}
}

func TestVerifyMissingAndCorruptFailReadOnly(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			d := filepath.Join(t.TempDir(), "missing")
			var before map[string][]byte
			if kind == "corrupt" {
				d = cliJournal(t)
				entries, _ := os.ReadDir(d)
				f, err := os.OpenFile(filepath.Join(d, entries[0].Name()), os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				f.WriteString("PRIVATE incomplete record")
				f.Close()
				before = snapshot(t, d)
			}
			var out, stderr bytes.Buffer
			if code := run([]string{"verify", d}, &out, &stderr); code != 1 {
				t.Fatalf("exit %d", code)
			}
			if out.Len() != 0 || !strings.Contains(stderr.String(), "audit verification failed") || strings.Contains(stderr.String(), "PRIVATE") {
				t.Fatalf("stdout=%s stderr=%s", out.String(), stderr.String())
			}
			if kind == "missing" {
				if _, err := os.Stat(d); !os.IsNotExist(err) {
					t.Fatal("created missing directory")
				}
			} else {
				after := snapshot(t, d)
				for p, b := range before {
					if !bytes.Equal(b, after[p]) {
						t.Fatal("repaired corrupt journal")
					}
				}
			}
		})
	}
}

func TestUsageExitStatus(t *testing.T) {
	for _, args := range [][]string{nil, {"verify"}, {"repair", "directory"}, {"verify", "directory", "extra"}} {
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("args %v code %d stdout=%s stderr=%s", args, code, out.String(), stderr.String())
		}
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("closed output") }
func TestOutputFailureExitStatus(t *testing.T) {
	d := cliJournal(t)
	var stderr bytes.Buffer
	if code := run([]string{"verify", d}, failedWriter{}, &stderr); code != 1 {
		t.Fatalf("exit %d", code)
	}
}
