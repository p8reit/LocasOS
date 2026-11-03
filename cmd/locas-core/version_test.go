package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestNumericVersionEligibilityAndTruthfulIdentity(t *testing.T) {
	cases := []struct {
		name, running, record, basis, comparison, availability string
		eligible                                               bool
	}{
		{"fresh newer", "v1.2.2", "", "running_release", "newer", "update_available", true},
		{"fresh same", "v1.2.3", "", "running_release", "same", "up_to_date", false},
		{"fresh older", "v1.10.0", "", "running_release", "older", "newer_installed", false},
		{"unknown legacy", "dev-abcd", "", "unknown", "unknown", "comparison_unknown", true},
		{"record newer", "dev-abcd", "v1.2.2", "verified_record", "newer", "update_available", true},
		{"record same", "dev-abcd", "v1.2.3", "verified_record", "same", "up_to_date", false},
		{"record older", "dev-abcd", "v1.2.4", "verified_record", "older", "newer_installed", false},
		{"invalid record", "dev-abcd", "corrupt", "invalid_state", "blocked", "version_state_invalid", false},
		{"conflicting record", "v1.2.2", "v1.2.1", "inconsistent", "blocked", "version_state_inconsistent", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUpdateFixture(t)
			f.u.runningVersion = tc.running
			f.u.runningCommit = strings.Repeat("d", 40)
			if tc.record != "" {
				if err := os.WriteFile(f.u.versionFile, []byte(tc.record+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			result := f.check(t)
			checkString := func(key, want string) {
				t.Helper()
				var value string
				if err := json.Unmarshal(result[key], &value); err != nil {
					t.Fatal(err)
				}
				if value != want {
					t.Fatalf("%s = %q want %q", key, value, want)
				}
			}
			checkString("installed_version", tc.running)
			checkString("installed_commit", f.u.runningCommit)
			checkString("version_basis", tc.basis)
			checkString("comparison", tc.comparison)
			checkString("availability", tc.availability)
			if (string(result["install_eligible"]) == "true") != tc.eligible || (string(result["install_ready"]) == "true") != tc.eligible {
				t.Fatalf("eligibility: %s", result)
			}
			if !tc.eligible && result["approval"] != nil {
				t.Fatal("ineligible release received one-use approval")
			}
			if strings.HasPrefix(tc.running, "dev") {
				checkString("installed_release", "")
			}
			if tc.record != "" && tc.record != "corrupt" {
				checkString("last_verified_release", tc.record)
			}
			info := f.req(t, "GET", "/api/updates", nil)
			if info.Code != 200 {
				t.Fatal(info.Code)
			}
			var identity map[string]string
			if err := json.Unmarshal(info.Body.Bytes(), &identity); err != nil {
				t.Fatal(err)
			}
			if identity["installed_version"] != tc.running || identity["installed_commit"] != f.u.runningCommit || identity["version_basis"] != tc.basis {
				t.Fatal("info and discovery identities diverge")
			}
		})
	}
}

func TestEligibilityRecheckedAndInvalidatedAtEnqueue(t *testing.T) {
	for _, record := range []string{"v1.2.3", "v9.0.0", "corrupt"} {
		t.Run(record, func(t *testing.T) {
			f := newUpdateFixture(t)
			approval := selected(t, f.check(t))
			if err := os.WriteFile(f.u.versionFile, []byte(record+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if f.req(t, "POST", "/api/updates/install", approval).Code != 409 || f.calls != 0 {
				t.Fatal("stale eligibility queued")
			}
			if err := os.Remove(f.u.versionFile); err != nil {
				t.Fatal(err)
			}
			if f.req(t, "POST", "/api/updates/install", approval).Code != 409 {
				t.Fatal("invalidated approval resurrected")
			}
		})
	}
}

func TestEligibilityRecheckedAfterDiscoveryIO(t *testing.T) {
	f := newUpdateFixture(t)
	f.u.runningVersion = "v1.2.2"
	original := f.u.client.Transport
	started := make(chan struct{})
	resume := make(chan struct{})
	var once sync.Once
	f.u.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/locasos-update.sig") {
			once.Do(func() { close(started); <-resume })
		}
		return original.RoundTrip(r)
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.req(t, "GET", "/api/updates/check", nil) }()
	<-started
	// Simulate a completed update record changing while discovery is on the wire.
	if err := os.WriteFile(f.u.versionFile, []byte("v1.2.3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	close(resume)
	w := <-done
	var result map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &result)
	if string(result["install_eligible"]) != "false" || string(result["install_ready"]) != "false" || result["approval"] != nil {
		t.Fatal("discovery used stale installed state")
	}
}
