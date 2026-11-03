package updatetrust

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InstalledState distinguishes the running binary from the last completed
// signed update record. A record alone is not proof of the running build.
type InstalledState struct {
	RunningVersion   string
	InstalledRelease string
	RecordedVersion  string
	Basis            string
	invalidReason    string
}

type Eligibility struct {
	Comparison   string
	Availability string
	Eligible     bool
	Reason       string
}

func InspectInstalled(runningVersion, recordFile string) InstalledState {
	state := InstalledState{RunningVersion: runningVersion, Basis: "unknown"}
	if _, ok := VersionParts(runningVersion); ok {
		state.InstalledRelease = runningVersion
		state.Basis = "running_release"
	}
	recorded, err := readVersionRecord(recordFile)
	if err != nil {
		state.Basis = "invalid_state"
		return state
	}
	state.RecordedVersion = recorded
	if state.InstalledRelease == "" && strings.HasPrefix(strings.TrimSpace(runningVersion), "v") {
		state.Basis = "invalid_state"
		state.invalidReason = "The running release identity is invalid. Use verified recovery before installing."
		return state
	}
	if recorded != "" {
		if state.InstalledRelease != "" && state.InstalledRelease != recorded {
			state.Basis = "inconsistent"
		} else if state.InstalledRelease == "" {
			state.Basis = "verified_record"
		}
	}
	return state
}

func (s InstalledState) Eligibility(target string) Eligibility {
	if _, ok := VersionParts(target); !ok {
		return Eligibility{"blocked", "version_state_invalid", false, "Release version is invalid."}
	}
	switch s.Basis {
	case "invalid_state":
		if s.invalidReason != "" {
			return Eligibility{"blocked", "version_state_invalid", false, s.invalidReason}
		}
		return Eligibility{"blocked", "version_state_invalid", false, "The verified update record is unreadable or invalid. Repair version state before installing."}
	case "inconsistent":
		return Eligibility{"blocked", "version_state_inconsistent", false, "The running release and last verified update differ. Wait for the current upgrade or use verified recovery."}
	}
	floor := s.InstalledRelease
	if floor == "" {
		floor = s.RecordedVersion
	}
	if floor == "" {
		return Eligibility{"unknown", "comparison_unknown", true, "Installed release version is unknown. This signed target is not confirmed newer; verify the intended replacement before approving."}
	}
	cmp, err := CompareVersions(target, floor)
	if err != nil {
		return Eligibility{"blocked", "version_state_invalid", false, "Cannot compare installed and target versions."}
	}
	if cmp == 0 {
		return Eligibility{"same", "up_to_date", false, "This release version is already installed or recorded. Same-version repair requires manual verified recovery."}
	}
	if cmp < 0 {
		return Eligibility{"older", "newer_installed", false, "A newer release version is installed or recorded. Dashboard downgrades are blocked."}
	}
	reason := "A numerically newer release is available. Signed artifact verification and explicit approval are still required."
	if s.Basis == "verified_record" {
		reason += " Comparison uses the last verified update record; the running build has no release tag."
	}
	return Eligibility{"newer", "update_available", true, reason}
}

func readVersionRecord(filename string) (string, error) {
	f, err := openVersionRecord(filename)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64 {
		return "", errors.New("invalid installed version record type or size")
	}
	data, err := io.ReadAll(io.LimitReader(f, 65))
	if err != nil {
		return "", err
	}
	if len(data) > 64 {
		return "", errors.New("installed version record too large")
	}
	version := strings.TrimSpace(string(data))
	if _, ok := VersionParts(version); !ok {
		return "", errors.New("invalid installed version record")
	}
	return version, nil
}

// PersistInstalledVersion runs only after successful health acceptance. Temp
// contents and the rename directory are synced before success is reported.
func PersistInstalledVersion(filename, version string) error {
	if _, ok := VersionParts(version); !ok {
		return errors.New("invalid installed version")
	}
	dir := filepath.Dir(filename)
	f, err := os.CreateTemp(dir, ".installed-version-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0644); err != nil {
		return err
	}
	if _, err := f.WriteString(version + "\n"); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), filename); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
