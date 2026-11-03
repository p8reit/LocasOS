package main

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/p8reit/LocasOS/internal/admin"
	"github.com/p8reit/LocasOS/internal/auditlog"
	"github.com/p8reit/LocasOS/internal/auditop"
)

type coreRecorder struct {
	events []auditlog.Event
	failAt int
}

func (r *coreRecorder) Append(e auditlog.Event) error {
	r.events = append(r.events, e)
	if len(r.events) == r.failAt {
		return errors.New("injected")
	}
	return nil
}
func coreRequest(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "localhost:59420"
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Locas-Request", "1")
	r.Header.Set("X-Locas-Audit-Correlation", strings.Repeat("f", 32))
	r.Header.Set("X-Locas-Audit-Session", "browser-secret")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestActualAuthRoutesDurabilityAndRedaction(t *testing.T) {
	dir := t.TempDir()
	a, err := admin.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	j, err := auditlog.Open(filepath.Join(dir, "audit"), "core")
	if err != nil {
		t.Fatal(err)
	}
	u := newUpdatesController(a, dir, "missing", "")
	h := coreAuditHandler(a, j, coreRoutes(a, "missing", u))
	password := "unique-password-not-in-audit"
	body := `{"password":"` + password + `"}`
	if w := coreRequest(h, "POST", "/api/setup", body, nil); w.Code != 201 {
		t.Fatalf("setup %d %s", w.Code, w.Body)
	}
	if w := coreRequest(h, "POST", "/api/login", `{"password":"wrong-secret"}`, nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := coreRequest(h, "POST", "/api/login", body, nil)
	if w.Code != 200 || len(w.Result().Cookies()) != 1 {
		t.Fatalf("login %d %s", w.Code, w.Body)
	}
	cookie := w.Result().Cookies()[0]
	sid := a.AuditIDForToken(cookie.Value)
	if len(sid) != 32 || strings.Contains(cookie.Value, sid) {
		t.Fatal("credential dependent audit reference")
	}
	if w := coreRequest(h, "POST", "/api/logout", "", cookie); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if a.AuditIDForToken(cookie.Value) != "" {
		t.Fatal("logged-out audit session still active")
	}
	report, err := auditlog.Verify(filepath.Join(dir, "audit"))
	if err != nil || report.Entries != 8 || len(report.Pending) != 0 {
		t.Fatalf("report %+v %v", report, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "audit", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{password, "wrong-secret", cookie.Value, "browser-secret", strings.Repeat("f", 32)} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatalf("secret/spoof retained: %q", secret)
		}
	}
	if !bytes.Contains(data, []byte(sid)) {
		t.Fatal("missing session correlation")
	}
}
func TestActualAuthRoutesAuditFailures(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			dir := t.TempDir()
			a, _ := admin.New(dir)
			rec := &coreRecorder{failAt: failAt}
			u := newUpdatesController(a, dir, "missing", "")
			h := coreAuditHandler(a, rec, coreRoutes(a, "missing", u))
			w := coreRequest(h, "POST", "/api/setup", `{"password":"valid-long-password"}`, nil)
			if w.Code != 503 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), "configured") {
				t.Fatalf("false success %d %s", w.Code, w.Body)
			}
			if a.Configured() != (failAt == 2) {
				t.Fatal("intent did not gate setup")
			}
		})
	}
	dir := t.TempDir()
	a, _ := admin.New(dir)
	_ = a.Setup("valid-long-password")
	rec := &coreRecorder{failAt: 2}
	u := newUpdatesController(a, dir, "missing", "")
	h := coreAuditHandler(a, rec, coreRoutes(a, "missing", u))
	w := coreRequest(h, "POST", "/api/login", `{"password":"valid-long-password"}`, nil)
	if w.Code != 503 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), "authenticated") {
		t.Fatal("login cookie/success released before outcome")
	}
}
func TestHeadDiscoveryCannotBypassIntent(t *testing.T) {
	f := newUpdateFixture(t)
	r := &coreRecorder{failAt: 1}
	h := coreAuditHandler(f.u.admins, r, f.u.Handler())
	w := coreRequest(h, "HEAD", "/api/updates/check", "", f.cookie)
	if w.Code != 503 || len(r.events) != 1 || r.events[0].Operation != "update.discover" {
		t.Fatalf("HEAD unaudited %d %+v", w.Code, r.events)
	}
}
func TestAppProxyUsesServerIdentityAndReportsAmbiguity(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "agent.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan auditop.Attribution, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, e := auditop.ReadHeaders(r)
		if e != nil {
			t.Error(e)
		}
		seen <- a
		w.WriteHeader(202)
	})}
	go srv.Serve(l)
	defer srv.Close()
	state := &auditop.State{Attribution: auditop.Attribution{CorrelationID: strings.Repeat("a", 32), Principal: "administrator", SessionID: strings.Repeat("b", 32)}, Operation: "app.install", Source: "browser"}
	r := httptest.NewRequest("POST", "/api/apps/install", nil)
	r.Header.Set("X-Locas-Audit-Correlation", "spoof")
	r = r.WithContext(auditop.With(r.Context(), state))
	w := httptest.NewRecorder()
	appProxy(w, r, socket, "POST", "/internal/apps/install", []byte(`{"app":"ollama"}`))
	if w.Code != 202 || !(<-seen).SameAssertion(state.Attribution) {
		t.Fatal("identity mismatch")
	}
	w = httptest.NewRecorder()
	appProxy(w, r, socket+"missing", "POST", "/internal/apps/install", nil)
	if w.Code != 503 || state.Outcome != "uncertain" || !strings.Contains(w.Body.String(), "check operation status") {
		t.Fatal("ambiguous submission mislabeled")
	}
}

func TestEverySecurityRouteRequiresDurableIntent(t *testing.T) {
	a, err := admin.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct{ method, path, operation string }{
		{"POST", "/api/setup", "auth.setup"}, {"POST", "/api/login", "auth.login"}, {"POST", "/api/logout", "auth.logout"},
		{"POST", "/api/model/pull", "model.pull"}, {"POST", "/api/chat", "model.chat"}, {"POST", "/api/apps/install", "app.install"},
		{"POST", "/api/updates/channel", "update.channel"}, {"POST", "/api/updates/install", "update.install"},
		{"GET", "/api/updates/check", "update.discover"}, {"HEAD", "/api/updates/check", "update.discover"},
		{"GET", "/api/runtime", "request.denied"}, {"HEAD", "/api/telemetry", "request.denied"}, {"DELETE", "/api/unknown", "request.denied"},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			recorder := &coreRecorder{failAt: 1}
			called := false
			h := coreAuditHandler(a, recorder, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
			w := coreRequest(h, route.method, route.path+"?secret-query=private", "private body", nil)
			if called || w.Code != 503 || len(recorder.events) != 1 || recorder.events[0].Operation != route.operation {
				t.Fatalf("admission failure bypass %+v %d", recorder.events, w.Code)
			}
		})
	}
	rec := &coreRecorder{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://evil.invalid/private-secret", nil)
	auditHostDenied(a, rec, w, r)
	if w.Code != 400 || len(rec.events) != 2 || rec.events[1].Outcome != "denied" {
		t.Fatalf("host denial %+v", rec.events)
	}
}

func TestDownstreamAuditOrResponseFailureRemainsUncertain(t *testing.T) {
	for _, mode := range []string{"audit-failure", "partial-response", "oversize-response"} {
		t.Run(mode, func(t *testing.T) {
			socket := filepath.Join(t.TempDir(), "agent.sock")
			l, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "audit-failure":
					http.Error(w, "audit outcome unavailable after enqueue", 503)
				case "partial-response":
					w.Header().Set("Content-Length", "100")
					w.WriteHeader(202)
					_, _ = w.Write([]byte("partial"))
				case "oversize-response":
					w.WriteHeader(202)
					_, _ = w.Write([]byte(strings.Repeat("x", 8193)))
				}
			})}
			go srv.Serve(l)
			defer srv.Close()
			state := &auditop.State{Attribution: auditop.Attribution{CorrelationID: strings.Repeat("a", 32), Principal: "administrator", SessionID: strings.Repeat("b", 32)}, Operation: "app.install", Source: "browser"}
			rec := &coreRecorder{}
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auditop.Serve(rec, "core", state, w, r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					appProxy(w, r, socket, "POST", "/internal/apps/install", nil)
				}))
			})
			w := coreRequest(h, "POST", "/api/apps/install", "", nil)
			if w.Code != 503 || len(rec.events) != 2 || rec.events[1].Outcome != "uncertain" || !strings.Contains(w.Body.String(), "check operation status") {
				t.Fatalf("misleading result %d %+v %s", w.Code, rec.events, w.Body)
			}
		})
	}
}
