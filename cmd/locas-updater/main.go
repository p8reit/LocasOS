package main

import (
	"archive/tar"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

// An official release pins its trusted publisher key at build time.
// Unsigned developer builds leave this empty and refuse privileged updates.
var trustedPublicKeyBase64 string

const (
	requestFile = "/var/lib/locas/update-install.request"
	stateDir    = "/var/lib/locas-updates"
	apiRoot     = "https://api.github.com/repos/p8reit/LocasOS/releases/tags/"
	releaseRoot = "https://github.com/p8reit/LocasOS/releases/download/"
)

var tagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type request struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

type status struct {
	State   string `json:"state"`
	Version string `json:"version,omitempty"`
	Channel string `json:"channel,omitempty"`
	Message string `json:"message,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if os.Geteuid() != 0 {
		return errors.New("updater requires root")
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(stateDir, "update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("update already running")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	buf, err := os.ReadFile(requestFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.Remove(requestFile); err != nil {
		return err
	}
	var req request
	if len(buf) > 1024 || json.Unmarshal(buf, &req) != nil ||
		!tagPattern.MatchString(req.Version) || (req.Channel != "stable" && req.Channel != "beta") {
		setStatus(status{State: "failed", Message: "invalid queued update request"})
		return errors.New("invalid update request")
	}
	fail := func(err error) error {
		setStatus(status{State: "failed", Version: req.Version, Channel: req.Channel, Message: err.Error()})
		return err
	}
	key, err := base64.StdEncoding.DecodeString(trustedPublicKeyBase64)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return fail(errors.New("trusted publisher key not configured; update blocked"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	client := &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) > 6 {
				return errors.New("too many release redirects")
			}
			host := r.URL.Hostname()
			if r.URL.Scheme != "https" || (host != "github.com" && !strings.HasSuffix(host, ".githubusercontent.com")) {
				return errors.New("untrusted release redirect")
			}
			return nil
		},
	}
	setStatus(status{State: "checking", Version: req.Version, Channel: req.Channel})
	metadata, err := fetch(ctx, client, apiRoot+req.Version, 1024*1024)
	if err != nil {
		return fail(err)
	}
	var rel release
	if err := json.Unmarshal(metadata, &rel); err != nil {
		return fail(err)
	}
	if rel.Tag != req.Version || rel.Draft || rel.Prerelease != (req.Channel == "beta") {
		return fail(errors.New("release channel or identity does not match approval"))
	}
	known := map[string]asset{}
	for _, item := range rel.Assets {
		switch item.Name {
		case "locasos-update.json", "locasos-update.sig", "locasos-upgrade-amd64.tar":
			expected := releaseRoot + req.Version + "/" + item.Name
			if item.URL == expected && item.Size > 0 && item.Size <= 512*1024*1024 {
				known[item.Name] = item
			}
		}
	}
	for _, name := range []string{"locasos-update.json", "locasos-update.sig", "locasos-upgrade-amd64.tar"} {
		if _, ok := known[name]; !ok {
			return fail(fmt.Errorf("signed release missing %s", name))
		}
	}
	work, err := os.MkdirTemp(stateDir, "stage-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(work)
	setStatus(status{State: "downloading", Version: req.Version, Channel: req.Channel})
	for _, name := range []string{"locasos-update.json", "locasos-update.sig", "locasos-upgrade-amd64.tar"} {
		item := known[name]
		max := item.Size + 1
		if name == "locasos-update.json" && max > 4096 {
			max = 4096
		}
		if name == "locasos-update.sig" && max > ed25519.SignatureSize {
			max = ed25519.SignatureSize
		}
		if err := download(ctx, client, item.URL, filepath.Join(work, name), max); err != nil {
			return fail(err)
		}
	}
	setStatus(status{State: "verifying", Version: req.Version, Channel: req.Channel})
	manifestBytes, err := os.ReadFile(filepath.Join(work, "locasos-update.json"))
	if err != nil {
		return fail(err)
	}
	signature, err := os.ReadFile(filepath.Join(work, "locasos-update.sig"))
	if err != nil {
		return fail(err)
	}
	manifest, err := updatetrust.Verify(manifestBytes, signature, ed25519.PublicKey(key))
	if err != nil {
		return fail(err)
	}
	if manifest.Version != req.Version {
		return fail(errors.New("signed release version mismatch"))
	}
	archive := filepath.Join(work, "locasos-upgrade-amd64.tar")
	digest, err := updatetrust.FileHash(archive)
	if err != nil {
		return fail(err)
	}
	if !strings.EqualFold(digest, manifest.SHA256) {
		return fail(errors.New("signed archive digest mismatch"))
	}
	if err := validateArchive(archive); err != nil {
		return fail(err)
	}
	sidecar := filepath.Join(work, "locasos-upgrade-amd64.tar.sha256")
	if err := os.WriteFile(sidecar, []byte(digest+"  locasos-upgrade-amd64.tar\n"), 0600); err != nil {
		return fail(err)
	}
	setStatus(status{State: "applying", Version: req.Version, Channel: req.Channel, Message: "Backup, install and service health checks"})
	command := exec.CommandContext(ctx, "bash", "/usr/local/sbin/locas-upgrade", archive)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fail(fmt.Errorf("upgrade failed; recovery attempted: %w", err))
	}
	if err := os.WriteFile(filepath.Join(stateDir, "current-version"), []byte(req.Version+"\n"), 0644); err != nil {
		return fail(err)
	}
	setStatus(status{State: "installed", Version: req.Version, Channel: req.Channel, Message: "Health-checked upgrade completed"})
	return nil
}

func fetch(ctx context.Context, client *http.Client, url string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "LOCAS-OS-signed-updater")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release download returned HTTP %d", res.StatusCode)
	}
	out, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > max {
		return nil, errors.New("release resource too large")
	}
	return out, nil
}

func download(ctx context.Context, client *http.Client, url, destination string, max int64) error {
	content, err := fetch(ctx, client, url, max)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, content, 0600)
}

func validateArchive(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := tar.NewReader(file)
	found := false
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(header.Name, "./"), "/")
		clean := path.Clean(name)
		if name == "." || name == "" || strings.HasPrefix(header.Name, "/") || clean != name || strings.HasPrefix(name, "../") {
			return errors.New("unsafe path in update archive")
		}
		if !(name == "opt" || name == "opt/locas" || name == "opt/locas/bin" ||
			strings.HasPrefix(name, "opt/locas/bin/") ||
			name == "etc" || name == "etc/systemd" || name == "etc/systemd/system" ||
			strings.HasPrefix(name, "etc/systemd/system/") ||
			name == "usr" || name == "usr/local" || name == "usr/local/sbin" ||
			strings.HasPrefix(name, "usr/local/sbin/")) {
			return fmt.Errorf("unauthorized update path %s", name)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && header.Typeflag != tar.TypeDir {
			return errors.New("links and special files forbidden in update archive")
		}
		if name == "opt/locas/bin/locas-core" && header.Typeflag == tar.TypeReg {
			found = true
		}
	}
	if !found {
		return errors.New("missing LOCAS core executable")
	}
	return nil
}

func setStatus(s status) {
	_ = os.MkdirAll(stateDir, 0755)
	tmp, err := os.CreateTemp(stateDir, "status-")
	if err != nil {
		return
	}
	data, _ := json.Marshal(s)
	_, _ = tmp.Write(append(data, '\n'))
	_ = tmp.Chmod(0644)
	_ = tmp.Close()
	_ = os.Rename(tmp.Name(), filepath.Join(stateDir, "status.json"))
}
