package updatetrust

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestInstalledIdentityAndEligibility(t *testing.T) {
	cases := []struct {
		running, record, target, basis, comparison, availability string
		eligible                                                 bool
	}{
		{"v1.2.3", "", "v1.2.4", "running_release", "newer", "update_available", true},
		{"v1.2.3", "", "v1.2.3", "running_release", "same", "up_to_date", false},
		{"v1.2.3", "", "v1.2.2", "running_release", "older", "newer_installed", false},
		{"v1.9.0", "v1.9.0", "v1.10.0", "running_release", "newer", "update_available", true},
		{"v1.10.0", "v1.10.0", "v1.9.0", "running_release", "older", "newer_installed", false},
		{"dev-abcd", "v2.0.0", "v1.0.0", "verified_record", "older", "newer_installed", false},
		{"dev-abcd", "v2.0.0", "v2.0.0", "verified_record", "same", "up_to_date", false},
		{"dev-abcd", "v2.0.0", "v2.1.0", "verified_record", "newer", "update_available", true},
		{"dev-abcd", "", "v1.2.3", "unknown", "unknown", "comparison_unknown", true},
		{"dev", "garbage", "v1.2.3", "invalid_state", "blocked", "version_state_invalid", false},
		{"v1.2.3", "v1.2.4", "v1.2.5", "inconsistent", "blocked", "version_state_inconsistent", false},
		{"v1.2.4", "v1.2.3", "v1.2.5", "inconsistent", "blocked", "version_state_inconsistent", false},
		{"dev", "", "not-a-version", "unknown", "blocked", "version_state_invalid", false},
	}
	for _, tc := range cases {
		t.Run(tc.running+"_"+tc.record+"_"+tc.target, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "current-version")
			if tc.record != "" {
				if err := os.WriteFile(file, []byte(tc.record+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			state := InspectInstalled(tc.running, file)
			policy := state.Eligibility(tc.target)
			if state.Basis != tc.basis || policy.Comparison != tc.comparison || policy.Availability != tc.availability || policy.Eligible != tc.eligible {
				t.Fatalf("state %#v policy %#v", state, policy)
			}
			if strings.HasPrefix(tc.running, "dev") && state.InstalledRelease != "" {
				t.Fatal("root record misrepresented as running release")
			}
		})
	}
}

func TestInstalledVersionRecordFailures(t *testing.T) {
	for _, kind := range []string{"empty", "oversize", "directory", "symlink", "dangling symlink", "parent not directory"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "current-version")
			switch kind {
			case "empty":
				os.WriteFile(file, nil, 0600)
			case "oversize":
				os.WriteFile(file, []byte(strings.Repeat("a", 65)), 0600)
			case "directory":
				os.Mkdir(file, 0700)
			case "symlink":
				target := filepath.Join(dir, "target")
				os.WriteFile(target, []byte("v1.2.3\n"), 0600)
				if err := os.Symlink(target, file); err != nil {
					t.Fatal(err)
				}
			case "dangling symlink":
				if err := os.Symlink(filepath.Join(dir, "missing"), file); err != nil {
					t.Fatal(err)
				}
			case "parent not directory":
				os.WriteFile(filepath.Join(dir, "parent"), []byte("file"), 0600)
				file = filepath.Join(dir, "parent", "current-version")
			}
			state := InspectInstalled("v1.2.3", file)
			if state.Basis != "invalid_state" || state.Eligibility("v9.0.0").Eligible {
				t.Fatalf("invalid state admitted: %#v", state)
			}
		})
	}
}

func TestAtomicVersionPersistenceAndFailure(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "current-version")
	if err := PersistInstalledVersion(file, "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	if value, err := readVersionRecord(file); err != nil || value != "v1.2.3" {
		t.Fatalf("%q %v", value, err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatal("record not readable by Core")
	}
	if err := PersistInstalledVersion(file, "invalid"); err == nil {
		t.Fatal("invalid record written")
	}
	if value, _ := readVersionRecord(file); value != "v1.2.3" {
		t.Fatal("failed write changed prior record")
	}
	if err := PersistInstalledVersion(filepath.Join(dir, "missing", "record"), "v1.2.4"); err == nil {
		t.Fatal("missing directory write succeeded")
	}
	targetDir := filepath.Join(dir, "directory")
	os.Mkdir(targetDir, 0700)
	if err := PersistInstalledVersion(targetDir, "v1.2.4"); err == nil {
		t.Fatal("directory replaced")
	}
	temps, _ := filepath.Glob(filepath.Join(dir, ".installed-version-*"))
	if len(temps) != 0 {
		t.Fatal("failed write leaked staged state")
	}
}

func TestReadersNeverObservePartialVersionWrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "current-version")
	if err := PersistInstalledVersion(file, "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				value, err := readVersionRecord(file)
				if err != nil || (value != "v1.0.0" && value != "v2.0.0") {
					t.Errorf("partial read %q %v", value, err)
					return
				}
			}
		}
	}()
	for i := 0; i < 20; i++ {
		version := "v1.0.0"
		if i%2 == 0 {
			version = "v2.0.0"
		}
		if err := PersistInstalledVersion(file, version); err != nil {
			t.Fatal(err)
		}
	}
	close(done)
	wg.Wait()
}

func TestMalformedReleaseIdentityNeverBecomesUnknownLegacy(t *testing.T) {
	for _, version := range []string{"v999999999999999999999999.0.0", "v1.bad.0", "v" + strings.Repeat("0", 64) + "1.0.0", " v1.2.3"} {
		t.Run(version, func(t *testing.T) {
			state := InspectInstalled(version, filepath.Join(t.TempDir(), "missing"))
			policy := state.Eligibility("v1.0.0")
			if state.Basis != "invalid_state" || policy.Eligible || policy.Comparison != "blocked" {
				t.Fatalf("malformed release admitted as unknown: %#v %#v", state, policy)
			}
		})
	}
}
func TestVersionRecordAndManifestShareNumericBounds(t *testing.T) {
	for _, version := range []string{"v999999999999999999999999.0.0", "v" + strings.Repeat("0", 64) + "1.0.0"} {
		if _, ok := VersionParts(version); ok {
			t.Fatal("unsupported version accepted")
		}
		m := Manifest{Version: version, Platform: "linux-amd64-wsl2", File: "locasos-upgrade-amd64.tar", Commit: strings.Repeat("a", 40), SHA256: strings.Repeat("b", 64)}
		if err := m.Validate(); err == nil {
			t.Fatal("manifest accepts unrepresentable version")
		}
		if err := PersistInstalledVersion(filepath.Join(t.TempDir(), "version"), version); err == nil {
			t.Fatal("unreadable version persisted")
		}
	}
}

func TestHealthAcceptanceRecordCompletesTransientMismatch(t *testing.T) {
	file := filepath.Join(t.TempDir(), "current-version")
	if err := PersistInstalledVersion(file, "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if state := InspectInstalled("v2.0.0", file); state.Basis != "inconsistent" || state.Eligibility("v3.0.0").Eligible {
		t.Fatal("in-flight mismatch not blocked")
	}
	if err := PersistInstalledVersion(file, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	state := InspectInstalled("v2.0.0", file)
	if state.Basis != "running_release" || !state.Eligibility("v3.0.0").Eligible {
		t.Fatal("completed verified version not usable")
	}
}

func TestMaximumVersionRecordBoundaryRoundtrips(t *testing.T) {
	version := "v" + strings.Repeat("0", 57) + "1.0.0"
	if len(version) != 63 {
		t.Fatal("bad fixture")
	}
	file := filepath.Join(t.TempDir(), "version")
	if err := PersistInstalledVersion(file, version); err != nil {
		t.Fatal(err)
	}
	if value, err := readVersionRecord(file); err != nil || value != version {
		t.Fatalf("boundary roundtrip: %q %v", value, err)
	}
}
