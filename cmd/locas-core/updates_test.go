package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/p8reit/LocasOS/internal/admin"
	"github.com/p8reit/LocasOS/internal/updateapproval"
	"github.com/p8reit/LocasOS/internal/updatequeue"
	"github.com/p8reit/LocasOS/internal/updatetrust"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type updateFixture struct {
	u         *updatesController
	cookie    *http.Cookie
	rel       releaseInfo
	manifest  []byte
	signature []byte
	private   ed25519.PrivateKey
	calls     int
	job       updateapproval.Job
}

func newUpdateFixture(t *testing.T) *updateFixture {
	t.Helper()
	dir := t.TempDir()
	admins, err := admin.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := admins.Setup("local-test-password-long"); err != nil {
		t.Fatal(err)
	}
	token, ok := admins.Login("local-test-password-long", "127.0.0.1:1234")
	if !ok {
		t.Fatal("login")
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f := &updateFixture{private: key, cookie: &http.Cookie{Name: "locas_session", Value: token}, rel: releaseInfo{Tag: "v1.2.3", URL: releasePageRoot + "v1.2.3"}}
	f.sign(t, updatetrust.Manifest{Version: "v1.2.3", Platform: "linux-amd64-wsl2", File: "locasos-upgrade-amd64.tar", SHA256: strings.Repeat("a", 64), Commit: strings.Repeat("b", 40)})
	f.u = newUpdatesController(admins, dir, "unused", base64.StdEncoding.EncodeToString(pub))
	f.u.versionFile = filepath.Join(dir, "current-version")
	f.u.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		var body []byte
		switch r.URL.String() {
		case releasesAPI:
			body, _ = json.Marshal([]releaseInfo{f.rel})
		case releaseDownloadRoot + "v1.2.3/locasos-update.json":
			body = f.manifest
		case releaseDownloadRoot + "v1.2.3/locasos-update.sig":
			body = f.signature
		default:
			t.Errorf("unexpected URL %s", r.URL)
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: 200, ContentLength: int64(len(body)), Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})
	f.u.enqueue = func(w http.ResponseWriter, r *http.Request, data []byte) {
		f.calls++
		if err := json.Unmarshal(data, &f.job); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}
	return f
}
func (f *updateFixture) sign(t *testing.T, m updatetrust.Manifest) {
	t.Helper()
	payload, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	f.manifest = payload
	f.signature = ed25519.Sign(f.private, payload)
	f.rel.Assets = nil
	for _, name := range []string{"locasos-upgrade-amd64.tar", "locasos-upgrade-amd64.tar.sha256", "locasos-update.json", "locasos-update.sig", "Upgrade-LocasOS.ps1", "upgrade-locas.sh"} {
		size := int64(1)
		if name == "locasos-update.json" {
			size = int64(len(payload))
		}
		if name == "locasos-update.sig" {
			size = 64
		}
		f.rel.Assets = append(f.rel.Assets, releaseAsset{Name: name, URL: releaseDownloadRoot + f.rel.Tag + "/" + name, Size: size})
	}
}
func (f *updateFixture) req(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.AddCookie(f.cookie)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Locas-Request", "1")
	w := httptest.NewRecorder()
	f.u.Handler().ServeHTTP(w, r)
	return w
}
func (f *updateFixture) check(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	w := f.req(t, "GET", "/api/updates/check", nil)
	if w.Code != 200 {
		t.Fatalf("check status %d: %s", w.Code, w.Body.String())
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func selected(t *testing.T, result map[string]json.RawMessage) updateapproval.Selection {
	t.Helper()
	if string(result["install_ready"]) != "true" {
		t.Fatalf("not installable: %s", result)
	}
	var a updateapproval.Selection
	if err := json.Unmarshal(result["approval"], &a); err != nil {
		t.Fatal(err)
	}
	return a
}
func TestSignedApprovalRoundtripAndReplay(t *testing.T) {
	f := newUpdateFixture(t)
	a := selected(t, f.check(t))
	w := f.req(t, "POST", "/api/updates/install", a)
	if w.Code != 202 {
		t.Fatalf("install %d: %s", w.Code, w.Body.String())
	}
	if f.calls != 1 || f.job.Selection != a || f.job.Validate(time.Now()) != nil {
		t.Fatal("approved identity not preserved")
	}
	if f.req(t, "POST", "/api/updates/install", a).Code != 409 || f.calls != 1 {
		t.Fatal("replayed enqueue")
	}
}
func TestDiscoveryFailsClosed(t *testing.T) {
	for _, scenario := range []string{"missing key", "wrong key", "manifest tamper", "wrong version", "wrong platform", "wrong file", "wrong asset namespace", "duplicate asset", "wrong release URL", "oversized metadata", "bad signature size"} {
		t.Run(scenario, func(t *testing.T) {
			f := newUpdateFixture(t)
			switch scenario {
			case "missing key":
				f.u.publicKey = ""
			case "wrong key":
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				f.u.publicKey = base64.StdEncoding.EncodeToString(pub)
			case "manifest tamper":
				f.manifest[0] = ' '
			case "wrong version", "wrong platform", "wrong file":
				var m updatetrust.Manifest
				json.Unmarshal(f.manifest, &m)
				switch scenario {
				case "wrong version":
					m.Version = "v1.2.4"
				case "wrong platform":
					m.Platform = "native"
				case "wrong file":
					m.File = "other.tar"
				}
				f.sign(t, m)
			case "wrong asset namespace":
				f.rel.Assets[0].URL = releaseDownloadRoot + "v9.9.9/" + f.rel.Assets[0].Name
			case "duplicate asset":
				f.rel.Assets = append(f.rel.Assets, f.rel.Assets[0])
			case "wrong release URL":
				f.rel.URL += "/unexpected"
			case "oversized metadata":
				for i := range f.rel.Assets {
					if f.rel.Assets[i].Name == "locasos-update.json" {
						f.rel.Assets[i].Size = 4097
					}
				}
			case "bad signature size":
				for i := range f.rel.Assets {
					if f.rel.Assets[i].Name == "locasos-update.sig" {
						f.rel.Assets[i].Size = 63
					}
				}
			}
			w := f.req(t, "GET", "/api/updates/check", nil)
			if w.Code == 200 {
				var result map[string]json.RawMessage
				json.Unmarshal(w.Body.Bytes(), &result)
				if string(result["install_ready"]) != "false" || result["approval"] != nil {
					t.Fatal("untrusted approval minted")
				}
			} else if w.Code != 502 {
				t.Fatalf("unexpected status %d", w.Code)
			}
			if f.calls != 0 {
				t.Fatal("discovery installed an update")
			}
		})
	}
}
func TestApprovalMutationAndChannelChange(t *testing.T) {
	f := newUpdateFixture(t)
	a := selected(t, f.check(t))
	other := a
	other.SHA256 = strings.Repeat("c", 64)
	if f.req(t, "POST", "/api/updates/install", other).Code != 409 {
		t.Fatal("digest substitution")
	}
	if f.req(t, "POST", "/api/updates/channel", map[string]string{"channel": "beta"}).Code != 200 {
		t.Fatal("save")
	}
	if f.req(t, "POST", "/api/updates/channel", map[string]string{"channel": "stable"}).Code != 200 {
		t.Fatal("save")
	}
	if f.req(t, "POST", "/api/updates/install", a).Code != 409 {
		t.Fatal("old selection survived channel switch")
	}
}
func TestFailedEnqueueStillConsumesApproval(t *testing.T) {
	f := newUpdateFixture(t)
	a := selected(t, f.check(t))
	calls := 0
	f.u.enqueue = func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	if f.req(t, "POST", "/api/updates/install", a).Code != 503 {
		t.Fatal("expected IPC failure")
	}
	if f.req(t, "POST", "/api/updates/install", a).Code != 409 || calls != 1 {
		t.Fatal("uncertain attempt retried")
	}
}
func TestRecheckInvalidatesPriorSelection(t *testing.T) {
	f := newUpdateFixture(t)
	a := selected(t, f.check(t))
	b := selected(t, f.check(t))
	if a.Token == b.Token {
		t.Fatal("token reused")
	}
	if f.req(t, "POST", "/api/updates/install", a).Code != 409 {
		t.Fatal("old discovery retained")
	}
	if f.req(t, "POST", "/api/updates/install", b).Code != 202 {
		t.Fatal("fresh discovery rejected")
	}
}
func TestDelayedDiscoveryCannotSurviveChannelChangeOrLogout(t *testing.T) {
	for _, scenario := range []string{"channel", "logout"} {
		t.Run(scenario, func(t *testing.T) {
			f := newUpdateFixture(t)
			original := f.u.client.Transport
			started := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			f.u.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() == releasesAPI {
					once.Do(func() { close(started); <-release })
				}
				return original.RoundTrip(r)
			})
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- f.req(t, "GET", "/api/updates/check", nil) }()
			<-started
			if scenario == "channel" {
				f.req(t, "POST", "/api/updates/channel", map[string]string{"channel": "beta"})
				f.req(t, "POST", "/api/updates/channel", map[string]string{"channel": "stable"})
			} else {
				r := httptest.NewRequest("POST", "/api/logout", nil)
				r.AddCookie(f.cookie)
				f.u.revoke(r)
				f.u.admins.Logout(r)
			}
			close(release)
			w := <-done
			var result map[string]json.RawMessage
			json.Unmarshal(w.Body.Bytes(), &result)
			if string(result["install_ready"]) != "false" {
				t.Fatal("delayed authorization resurrected")
			}
		})
	}
}
func TestUpdateEndpointsRequireAuthenticationAndMutationHeader(t *testing.T) {
	f := newUpdateFixture(t)
	for _, path := range []string{"/api/updates", "/api/updates/check", "/api/updates/status", "/api/updates/install", "/api/updates/channel"} {
		w := httptest.NewRecorder()
		f.u.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatalf("%s bypass auth", path)
		}
	}
	r := httptest.NewRequest("POST", "/api/updates/install", strings.NewReader(`{}`))
	r.AddCookie(f.cookie)
	w := httptest.NewRecorder()
	f.u.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("unsafe mutation accepted")
	}
}
func TestReleaseFetchLimitsAndRedirects(t *testing.T) {
	for _, scenario := range []string{"oversize", "short", "timeout", "redirect"} {
		t.Run(scenario, func(t *testing.T) {
			f := newUpdateFixture(t)
			if scenario == "redirect" {
				r := httptest.NewRequest("GET", "https://evil.invalid/file", nil)
				if f.u.client.CheckRedirect(r, nil) == nil {
					t.Fatal("redirect accepted")
				}
				return
			}
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if scenario == "timeout" {
					return nil, context.DeadlineExceeded
				}
				body := "12"
				if scenario == "oversize" {
					body = "123456"
				}
				return &http.Response{StatusCode: 200, ContentLength: -1, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			if _, err := fetchReleaseResource(context.Background(), client, "https://github.com/file", 4, 4); err == nil {
				t.Fatal("invalid resource accepted")
			}
		})
	}
}

func TestApprovedIdentitySurvivesCoreToAgentQueue(t *testing.T) {
	f := newUpdateFixture(t)
	dir := t.TempDir()
	handler := updatequeue.New(dir).Handler()
	f.u.enqueue = func(w http.ResponseWriter, _ *http.Request, data []byte) {
		handler.ServeHTTP(w, httptest.NewRequest("POST", "/install", bytes.NewReader(data)))
	}
	a := selected(t, f.check(t))
	w := f.req(t, "POST", "/api/updates/install", a)
	if w.Code != 202 {
		t.Fatalf("queue status %d: %s", w.Code, w.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "update-install.request"))
	if err != nil {
		t.Fatal(err)
	}
	var job updateapproval.Job
	if err := updateapproval.Decode(bytes.NewReader(data), &job); err != nil {
		t.Fatal(err)
	}
	if job.Selection != a || job.Validate(time.Now()) != nil {
		t.Fatal("queued identity differs from approval")
	}
	if f.req(t, "POST", "/api/updates/install", a).Code != 409 {
		t.Fatal("queue replay")
	}
}
func TestSelectionCannotCrossSessionOrCoreRestart(t *testing.T) {
	f := newUpdateFixture(t)
	a := selected(t, f.check(t))
	original := f.cookie
	token, ok := f.u.admins.Login("local-test-password-long", "127.0.0.1:9999")
	if !ok {
		t.Fatal("second login")
	}
	f.cookie = &http.Cookie{Name: "locas_session", Value: token}
	if f.req(t, "POST", "/api/updates/install", a).Code != 409 {
		t.Fatal("cross-session approval")
	}
	f.cookie = original
	f.u.approvals = updateapproval.NewStore()
	if f.req(t, "POST", "/api/updates/install", a).Code != 409 {
		t.Fatal("restart reused approval")
	}
}

func TestLateRecheckCannotReplaceNewerApproval(t *testing.T) {
	f := newUpdateFixture(t)
	original := f.u.client.Transport
	started := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	first := true
	f.u.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == releasesAPI {
			mu.Lock()
			block := first
			first = false
			mu.Unlock()
			if block {
				close(started)
				<-release
			}
		}
		return original.RoundTrip(r)
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.req(t, "GET", "/api/updates/check", nil) }()
	<-started
	fresh := selected(t, f.check(t))
	close(release)
	old := <-done
	var result map[string]json.RawMessage
	json.Unmarshal(old.Body.Bytes(), &result)
	if string(result["install_ready"]) != "false" {
		t.Fatal("late check minted approval")
	}
	if f.req(t, "POST", "/api/updates/install", fresh).Code != 202 {
		t.Fatal("late check invalidated current approval")
	}
}
func TestGlobalChannelMutationInvalidatesOtherSession(t *testing.T) {
	f := newUpdateFixture(t)
	a := selected(t, f.check(t))
	firstCookie := f.cookie
	token, ok := f.u.admins.Login("local-test-password-long", "127.0.0.1:4444")
	if !ok {
		t.Fatal("login")
	}
	f.cookie = &http.Cookie{Name: "locas_session", Value: token}
	b := selected(t, f.check(t))
	f.req(t, "POST", "/api/updates/channel", map[string]string{"channel": "beta"})
	f.req(t, "POST", "/api/updates/channel", map[string]string{"channel": "stable"})
	if f.req(t, "POST", "/api/updates/install", b).Code != 409 {
		t.Fatal("current session survived preference change")
	}
	f.cookie = firstCookie
	if f.req(t, "POST", "/api/updates/install", a).Code != 409 {
		t.Fatal("other session survived preference change")
	}
}
