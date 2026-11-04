package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/p8reit/LocasOS/internal/appmanager"
	"github.com/p8reit/LocasOS/internal/auditlog"
	"github.com/p8reit/LocasOS/internal/auditop"
)

type memoryRecorder struct {
	events []auditlog.Event
	failAt int
}

func (m *memoryRecorder) Append(e auditlog.Event) error {
	if len(m.events)+1 == m.failAt {
		return errors.New("injected persistence failure")
	}
	m.events = append(m.events, e)
	return nil
}
func fixture(t *testing.T, app string) (*worker, *memoryRecorder, *[]string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "app-install.request")
	uid := uint32(os.Geteuid())
	peer := uint32(123)
	job := appmanager.Request{App: app, Audit: auditop.Attribution{CorrelationID: strings.Repeat("a", 32), Principal: "administrator", SessionID: strings.Repeat("b", 32), PeerUID: &peer}}
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0640); err != nil {
		t.Fatal(err)
	}
	recorder := &memoryRecorder{}
	calls := []string{}
	w := &worker{requestPath: path, statusDir: filepath.Join(dir, "status"), queueUID: uid, recorder: recorder, ollamaReady: func(context.Context) bool { return true }}
	w.execute = func(_ context.Context, path string, args ...string) error {
		if len(recorder.events) == 0 || recorder.events[0].Stage != "intent" {
			t.Fatal("action without durable intent")
		}
		calls = append(calls, path+" "+strings.Join(args, " "))
		return nil
	}
	return w, recorder, &calls
}
func status(t *testing.T, w *worker) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(w.statusDir, "app-install.status"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return appmanager.ParseStatus(strings.TrimSpace(string(data))).Job
}
func TestWorkerFixedApplicationCommandsAndCorrelation(t *testing.T) {
	for _, app := range []string{"ollama", "open-webui"} {
		t.Run(app, func(t *testing.T) {
			w, recorder, calls := fixture(t, app)
			if err := w.run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if status(t, w) != "installed:"+app || len(*calls) != 2 {
				t.Fatalf("status %s calls %v", status(t, w), *calls)
			}
			if len(recorder.events) != 2 || recorder.events[0].Stage != "intent" || recorder.events[1].Outcome != "succeeded" {
				t.Fatal("missing transaction audit")
			}
			for _, event := range recorder.events {
				if event.Service != "app-worker" || event.Source != "agent_queue" || event.CorrelationID != strings.Repeat("a", 32) || event.PeerUID == nil || *event.PeerUID != 123 || event.App != app {
					t.Fatal("correlation not preserved")
				}
			}
			if app == "ollama" && (*calls)[0] != "/usr/local/sbin/locas-install-ollama " {
				t.Fatal("unexpected Ollama command")
			}
			if app == "open-webui" && (*calls)[1] != "/usr/local/sbin/locas-install-open-webui " {
				t.Fatal("unexpected WebUI command")
			}
		})
	}
}
func TestWorkerExistingOllamaUsesValidatedFixedInstaller(t *testing.T) {
	w, _, calls := fixture(t, "ollama")
	w.ollamaReady = func(context.Context) bool { return true }
	if err := w.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if (*calls)[0] != "/usr/local/sbin/locas-install-ollama " {
		t.Fatalf("calls %v", *calls)
	}
}
func TestWorkerJournalFailureBeforeActionAndAfterAction(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			w, recorder, calls := fixture(t, "ollama")
			recorder.failAt = failAt
			if err := w.run(context.Background()); err == nil {
				t.Fatal("audit failure ignored")
			}
			if strings.HasPrefix(status(t, w), "installed:") {
				t.Fatal("normal success despite audit failure")
			}
			if failAt == 1 {
				if len(*calls) != 0 {
					t.Fatal("action despite intent failure")
				}
				if _, err := os.Stat(w.requestPath); err != nil {
					t.Fatal("intent failure consumed request")
				}
			} else {
				if len(*calls) != 2 || status(t, w) != "uncertain:ollama" {
					t.Fatal("missing uncertain outcome")
				}
			}
		})
	}
}
func TestWorkerPartialInstallerFailureIsUncertainWithoutRawError(t *testing.T) {
	w, recorder, _ := fixture(t, "ollama")
	sideEffect := filepath.Join(t.TempDir(), "partially-installed")
	w.execute = func(context.Context, string, ...string) error {
		if err := os.WriteFile(sideEffect, []byte("partial installer change"), 0600); err != nil {
			t.Fatal(err)
		}
		return errors.New("SECRET-CREDENTIAL-RAW-ERROR")
	}
	err := w.run(context.Background())
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("raw installer error returned")
	}
	if len(recorder.events) != 2 || recorder.events[1].Outcome != "uncertain" || status(t, w) != "uncertain:ollama" {
		t.Fatal("partial failure uncertainty not durably recorded")
	}
	if _, err := os.Stat(sideEffect); err != nil {
		t.Fatal("partial side effect fixture missing")
	}
	data, _ := json.Marshal(recorder.events)
	if strings.Contains(string(data), "SECRET") {
		t.Fatal("secret logged")
	}
}
func TestWorkerStrictQueueRejectsLegacyMalformedAndUnsafeFiles(t *testing.T) {
	for _, scenario := range []string{"plaintext", "missing attribution", "missing peer", "unknown field", "trailing JSON", "oversized", "symlink", "fifo", "wrong owner", "permissions", "unknown app"} {
		t.Run(scenario, func(t *testing.T) {
			w, recorder, calls := fixture(t, "ollama")
			data, _ := os.ReadFile(w.requestPath)
			switch scenario {
			case "plaintext":
				data = []byte("ollama\n")
			case "missing attribution":
				data = []byte(`{"app":"ollama"}`)
			case "missing peer":
				var req appmanager.Request
				_ = json.Unmarshal(data, &req)
				req.Audit.PeerUID = nil
				data, _ = json.Marshal(req)
			case "unknown field":
				data = append(data[:len(data)-1], []byte(`,"command":"/bin/sh"}`)...)
			case "trailing JSON":
				data = append(data, []byte(` {}`)...)
			case "oversized":
				data = []byte(strings.Repeat("x", 4097))
			case "symlink":
				_ = os.Remove(w.requestPath)
				target := w.requestPath + "-target"
				_ = os.WriteFile(target, data, 0640)
				if err := os.Symlink(target, w.requestPath); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				_ = os.Remove(w.requestPath)
				if err := syscall.Mkfifo(w.requestPath, 0600); err != nil {
					t.Fatal(err)
				}
			case "wrong owner":
				w.queueUID++
			case "permissions":
				_ = os.Chmod(w.requestPath, 0666)
			case "unknown app":
				var req appmanager.Request
				_ = json.Unmarshal(data, &req)
				req.App = "/bin/sh"
				data, _ = json.Marshal(req)
			}
			if scenario != "symlink" && scenario != "fifo" {
				if err := os.WriteFile(w.requestPath, data, 0640); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.run(context.Background()); err == nil {
				t.Fatal("bad queue accepted")
			}
			if len(*calls) != 0 || len(recorder.events) != 2 || recorder.events[1].Outcome != "denied" || status(t, w) != "rejected" {
				t.Fatal("invalid request executed or not audited")
			}
			for _, event := range recorder.events {
				if event.Principal != "anonymous" || event.App != "" || event.SessionID != "" {
					t.Fatal("invalid input retained in audit")
				}
			}
		})
	}
}
func TestWorkerRealJournalRestartRejectsReplayedCorrelation(t *testing.T) {
	w, _, calls := fixture(t, "ollama")
	dir := filepath.Join(t.TempDir(), "audit")
	journal, err := auditlog.Open(dir, "app-worker")
	if err != nil {
		t.Fatal(err)
	}
	w.recorder = journal
	// Executor observes durable intent from the actual journal, not a fake recorder.
	w.execute = func(context.Context, string, ...string) error {
		report, err := auditlog.Verify(dir)
		if err != nil || report.Entries < 1 || len(report.Pending) != 1 {
			t.Fatal("real durable intent absent")
		}
		*calls = append(*calls, "fixed")
		return nil
	}
	payload, _ := os.ReadFile(w.requestPath)
	if err = w.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	report, err := auditlog.Verify(dir)
	if err != nil || report.Entries != 2 || len(report.Pending) != 0 {
		t.Fatal("journal result absent")
	}
	if err = os.WriteFile(w.requestPath, payload, 0640); err != nil {
		t.Fatal(err)
	}
	w.recorder, err = auditlog.Open(dir, "app-worker")
	if err != nil {
		t.Fatal(err)
	}
	if err = w.run(context.Background()); err == nil {
		t.Fatal("replayed correlation accepted")
	}
	if len(*calls) != 2 {
		t.Fatal("replay executed installer")
	}
}
func TestWorkerRootLockPreventsConcurrentInstaller(t *testing.T) {
	w, _, calls := fixture(t, "ollama")
	if err := os.MkdirAll(w.statusDir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(w.statusDir, "worker.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err = w.run(context.Background()); err == nil || len(*calls) != 0 {
		t.Fatal("concurrent worker executed")
	}
}

func TestWorkerConsumesAttributedQueueManagerArtifact(t *testing.T) {
	w, recorder, calls := fixture(t, "ollama")
	if err := os.Remove(w.requestPath); err != nil {
		t.Fatal(err)
	}
	peer := uint32(123)
	attribution := auditop.Attribution{CorrelationID: strings.Repeat("c", 32), Principal: "administrator", SessionID: strings.Repeat("d", 32), PeerUID: &peer}
	r := httptest.NewRequest("POST", "/install", strings.NewReader(`{"app":"ollama"}`))
	r = r.WithContext(auditop.WithPeer(auditop.With(r.Context(), &auditop.State{Attribution: attribution, Operation: "app.install", Source: "core_ipc"}), peer))
	response := httptest.NewRecorder()
	appmanager.New(filepath.Dir(w.requestPath)).Handler().ServeHTTP(response, r)
	if response.Code != 202 {
		t.Fatalf("queue status %d", response.Code)
	}
	if err := w.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 || len(recorder.events) != 2 {
		t.Fatal("queue was not executed and audited")
	}
	for _, event := range recorder.events {
		if event.CorrelationID != attribution.CorrelationID || event.SessionID != attribution.SessionID || event.PeerUID == nil || *event.PeerUID != peer {
			t.Fatal("queue attribution did not round trip")
		}
	}
}

func TestRequestDecodeBoundsActualBytesAfterFileGrowth(t *testing.T) {
	w, _, _ := fixture(t, "ollama")
	valid, err := os.ReadFile(w.requestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{4096, 4097, 8192} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			data := string(valid) + strings.Repeat(" ", size-len(valid))
			req, err := decodeRequest(strings.NewReader(data))
			if size == 4096 {
				if err != nil || req.App != "ollama" {
					t.Fatal("bounded valid request rejected")
				}
			} else if err == nil {
				t.Fatal("grown request bypassed actual byte bound")
			}
		})
	}
}
