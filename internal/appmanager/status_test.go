package appmanager

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func appFixture(t *testing.T) *Manager {
	t.Helper()
	m := New(t.TempDir())
	m.StatusPath = filepath.Join(t.TempDir(), "status")
	m.BinaryPath = filepath.Join(t.TempDir(), "ollama")
	m.WorkspacePath = filepath.Join(t.TempDir(), "data")
	m.Ready = func(context.Context) bool { return false }
	return m
}
func TestClassifiedStatusAndLegacyCompatibility(t *testing.T) {
	for _, job := range []string{"idle", "rejected", "installing:ollama", "installed:open-webui", "failed:open-webui", "uncertain:open-webui"} {
		if ParseStatus(job).Job != job {
			t.Fatal(job)
		}
	}
	raw := "uncertain:open-webui:webui-docker-install:" + strings.Repeat("a", 32)
	s := ParseStatus(raw)
	if s.Job != "uncertain:open-webui" || s.Reason != "webui-docker-install" || len(s.StatusToken) != 64 {
		t.Fatalf("%+v", s)
	}
	if ParseStatus(raw+"\n").StatusToken != s.StatusToken || ParseStatus(strings.Replace(raw, "docker-install", "readiness", 1)).StatusToken == s.StatusToken {
		t.Fatal("snapshot token not bound to classified record")
	}
	for _, raw := range []string{"SECRET:raw-error", "uncertain:/etc/secret", "uncertain:open-webui:SECRET:" + strings.Repeat("a", 32), "uncertain:ollama:webui-pull:" + strings.Repeat("a", 32), strings.Repeat("x", 1000)} {
		data, _ := json.Marshal(ParseStatus(raw))
		if strings.Contains(string(data), "SECRET") || ParseStatus(raw).State != "unknown" {
			t.Fatal("unclassified state escaped")
		}
	}
}
func TestReadStatusRejectsUnsafeOrOversizedFile(t *testing.T) {
	for _, mode := range []string{"fifo", "symlink", "oversize", "directory"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "status")
			switch mode {
			case "fifo":
				if e := syscall.Mkfifo(path, 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				if e := os.Symlink("/etc/passwd", path); e != nil {
					t.Fatal(e)
				}
			case "oversize":
				_ = os.WriteFile(path, []byte(strings.Repeat("x", 1000)), 0600)
			case "directory":
				_ = os.Mkdir(path, 0700)
			}
			if ReadStatus(path).State != "unknown" {
				t.Fatal("unsafe status read")
			}
		})
	}
}
func TestUncertainInstallNeedsCurrentAcknowledgement(t *testing.T) {
	for _, ack := range []string{"", strings.Repeat("0", 64), "current"} {
		t.Run(ack, func(t *testing.T) {
			m := appFixture(t)
			raw := "uncertain:ollama:ollama-readiness:" + strings.Repeat("a", 32)
			_ = os.WriteFile(m.StatusPath, []byte(raw+"\n"), 0600)
			if ack == "current" {
				ack = ParseStatus(raw).StatusToken
			}
			body, _ := json.Marshal(map[string]string{"app": "ollama", "acknowledge_status": ack})
			w := httptest.NewRecorder()
			m.Handler().ServeHTTP(w, trusted(httptest.NewRequest("POST", "/install", strings.NewReader(string(body)))))
			want := 409
			if ack == ParseStatus(raw).StatusToken {
				want = 202
			}
			if w.Code != want {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			data, err := os.ReadFile(filepath.Join(m.Dir, "app-install.request"))
			if want == 202 {
				if err != nil || strings.Contains(string(data), "acknowledge_status") || strings.Contains(string(data), ack) {
					t.Fatal("acknowledgement leaked into queue")
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("unacknowledged job queued")
			}
		})
	}
}
func TestWebUIRequiresLiveOllamaAndRejectsChangedSnapshot(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "not-ready", true: "changed"}[changed], func(t *testing.T) {
			m := appFixture(t)
			raw := "uncertain:open-webui:webui-pull:" + strings.Repeat("a", 32)
			_ = os.WriteFile(m.StatusPath, []byte(raw), 0600)
			m.Ready = func(context.Context) bool {
				if changed {
					_ = os.WriteFile(m.StatusPath, []byte("uncertain:open-webui:webui-readiness:"+strings.Repeat("b", 32)), 0600)
				}
				return changed
			}
			body, _ := json.Marshal(map[string]string{"app": "open-webui", "acknowledge_status": ParseStatus(raw).StatusToken})
			w := httptest.NewRecorder()
			m.Handler().ServeHTTP(w, trusted(httptest.NewRequest("POST", "/install", strings.NewReader(string(body)))))
			if w.Code != 409 {
				t.Fatalf("unsafe admission %d", w.Code)
			}
			if _, err := os.Stat(filepath.Join(m.Dir, "app-install.request")); !os.IsNotExist(err) {
				t.Fatal("unsafe job queued")
			}
		})
	}
}
func TestBinaryAndDataPresenceAreNotReadiness(t *testing.T) {
	m := appFixture(t)
	if err := os.WriteFile(m.BinaryPath, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(m.WorkspacePath, m.WorkspacePath); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/status", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ollama_installed"] != true || body["ollama_ready"] != false || body["workspace_state"] != "unknown" {
		t.Fatal(body)
	}
}

type readinessTransport func(*http.Request) (*http.Response, error)

func (f readinessTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRuntimeHealthIsBoundedAndClassified(t *testing.T) {
	for _, body := range []string{`{"version":"0.13.1"}`, `{"status":"ok"}`, `{"version":"SECRET /path"}`, strings.Repeat("x", 1025)} {
		client := &http.Client{Transport: readinessTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "http://127.0.0.1:11434/api/version" {
				t.Fatal(r.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if got := runtimeAPIReady(context.Background(), client); got != (body == `{"version":"0.13.1"}`) {
			t.Fatal("invalid health acceptance")
		}
	}
}
