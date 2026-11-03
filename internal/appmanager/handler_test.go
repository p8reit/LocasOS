package appmanager

import (
	"encoding/json"
	"github.com/p8reit/LocasOS/internal/auditop"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectUnknownApp(t *testing.T) {
	m := New(t.TempDir())
	r := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(`{"app":"bash"}`))
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, trusted(r))
	if w.Code != 400 {
		t.Fatalf("wanted 400 got %d", w.Code)
	}
}
func TestQueueAtMostOneJob(t *testing.T) {
	m := New(t.TempDir())
	for _, want := range []int{202, 409} {
		r := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(`{"app":"ollama"}`))
		w := httptest.NewRecorder()
		m.Handler().ServeHTTP(w, trusted(r))
		if w.Code != want {
			t.Fatalf("wanted %d got %d", want, w.Code)
		}
	}
}

func trusted(r *http.Request) *http.Request {
	uid := uint32(123)
	state := &auditop.State{Attribution: auditop.Attribution{CorrelationID: strings.Repeat("a", 32), Principal: "administrator", SessionID: strings.Repeat("b", 32), PeerUID: &uid}, Operation: "app.install", Source: "core_ipc"}
	return r.WithContext(auditop.WithPeer(auditop.With(r.Context(), state), uid))
}
func TestAppQueueCarriesOnlyTrustedAttribution(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	request := trusted(httptest.NewRequest("POST", "/install", strings.NewReader(`{"app":"ollama"}`)))
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, request)
	if w.Code != 202 {
		t.Fatalf("status %d", w.Code)
	}
	data, err := os.ReadFile(filepath.Join(dir, "app-install.request"))
	if err != nil {
		t.Fatal(err)
	}
	var job Request
	if err = json.Unmarshal(data, &job); err != nil {
		t.Fatal(err)
	}
	if job.App != "ollama" || job.Validate() != nil || !job.Audit.SameAssertion(auditop.From(request.Context()).Attribution) || *job.Audit.PeerUID != 123 {
		t.Fatal("lost trusted identity")
	}
}
func TestAppQueueRejectsMissingPeerAndBrowserAttribution(t *testing.T) {
	for _, body := range []string{`{"app":"ollama"}`, `{"app":"ollama","audit":{"principal":"administrator"}}`, `{"app":"ollama"} {}`} {
		dir := t.TempDir()
		r := httptest.NewRequest("POST", "/install", strings.NewReader(body))
		w := httptest.NewRecorder()
		New(dir).Handler().ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("missing context status %d", w.Code)
		}
		if _, err := os.Stat(filepath.Join(dir, "app-install.request")); !os.IsNotExist(err) {
			t.Fatal("unauthorized job queued")
		}
		r = trusted(httptest.NewRequest("POST", "/install", strings.NewReader(body)))
		w = httptest.NewRecorder()
		New(dir).Handler().ServeHTTP(w, r)
		if body != `{"app":"ollama"}` && w.Code != 400 {
			t.Fatalf("invalid body status %d", w.Code)
		}
	}
}
