package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/p8reit/LocasOS/internal/admin"
	"github.com/p8reit/LocasOS/internal/updateapproval"
	"github.com/p8reit/LocasOS/internal/updateprefs"
	"github.com/p8reit/LocasOS/internal/updatetrust"
)

// Populated only from trusted publisher configuration in the appliance build.
var trustedPublicKeyBase64 string

const releasesAPI = "https://api.github.com/repos/p8reit/LocasOS/releases?per_page=100"
const releaseDownloadRoot = "https://github.com/p8reit/LocasOS/releases/download/"
const releasePageRoot = "https://github.com/p8reit/LocasOS/releases/tag/"

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}
type releaseInfo struct {
	Tag        string         `json:"tag_name"`
	URL        string         `json:"html_url"`
	Published  string         `json:"published_at"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type updatesController struct {
	admins         *admin.Manager
	stateDir       string
	socket         string
	publicKey      string
	runningVersion string
	runningCommit  string
	versionFile    string
	client         *http.Client
	approvals      *updateapproval.Store
	channelMu      sync.Mutex // channel persistence and approval invalidation are atomic
	enqueue        func(http.ResponseWriter, *http.Request, []byte)
}

func newUpdatesController(admins *admin.Manager, stateDir, socket, key string) *updatesController {
	u := &updatesController{runningVersion: buildVersion, runningCommit: buildCommit, versionFile: "/var/lib/locas-updates/current-version", admins: admins, stateDir: stateDir, socket: socket, publicKey: key, approvals: updateapproval.NewStore(), client: &http.Client{
		Timeout: 6 * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			host := r.URL.Hostname()
			if len(via) > 6 || r.URL.Scheme != "https" || (host != "github.com" && !strings.HasSuffix(host, ".githubusercontent.com")) {
				return errors.New("untrusted release redirect")
			}
			return nil
		},
	}}
	u.enqueue = func(w http.ResponseWriter, r *http.Request, data []byte) {
		appProxy(w, r, socket, http.MethodPost, "/internal/updates/install", data)
	}
	return u
}

func (u *updatesController) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/updates", u.info)
	mux.HandleFunc("POST /api/updates/channel", u.channel)
	mux.HandleFunc("GET /api/updates/check", u.check)
	mux.HandleFunc("POST /api/updates/install", u.install)
	mux.HandleFunc("GET /api/updates/status", func(w http.ResponseWriter, r *http.Request) {
		appProxy(w, r, u.socket, http.MethodGet, "/internal/updates/status", nil)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !u.admins.Authenticated(r) {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost && !admin.SafeMutation(r) {
			http.Error(w, "invalid request", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (u *updatesController) revoke(r *http.Request) {
	u.approvals.Invalidate(u.admins.SessionBinding(r))
}
func (u *updatesController) identity(result map[string]any) updatetrust.InstalledState {
	state := updatetrust.InspectInstalled(u.runningVersion, u.versionFile)
	result["installed_version"] = u.runningVersion
	result["installed_commit"] = u.runningCommit
	result["installed_release"] = state.InstalledRelease
	result["last_verified_release"] = state.RecordedVersion
	result["version_basis"] = state.Basis
	return state
}
func (u *updatesController) eligibility(result map[string]any, target string) updatetrust.Eligibility {
	policy := u.identity(result).Eligibility(target)
	result["comparison"] = policy.Comparison
	result["availability"] = policy.Availability
	result["install_eligible"] = policy.Eligible
	result["eligibility_reason"] = policy.Reason
	return policy
}
func (u *updatesController) info(w http.ResponseWriter, r *http.Request) {
	channel, _ := updateprefs.Load(u.stateDir)
	result := map[string]any{"channel": channel, "availability": "not_checked", "update_method": "signed_github_release", "message": "Installation requires a signed artifact and one-time administrator approval of its SHA-256."}
	u.identity(result)
	sendJSON(w, http.StatusOK, result)
}
func (u *updatesController) channel(w http.ResponseWriter, r *http.Request) {
	var selection struct {
		Channel string `json:"channel"`
	}
	if err := updateapproval.Decode(http.MaxBytesReader(w, r.Body, 256), &selection); err != nil || !updateprefs.ValidChannel(selection.Channel) {
		http.Error(w, "unsupported update channel", http.StatusBadRequest)
		return
	}
	u.channelMu.Lock()
	defer u.channelMu.Unlock()
	u.approvals.InvalidateAll() // preference is appliance-wide, not session-local
	if err := updateprefs.Save(u.stateDir, selection.Channel); err != nil {
		http.Error(w, "cannot save update channel", http.StatusInternalServerError)
		return
	}
	sendJSON(w, http.StatusOK, map[string]string{"channel": selection.Channel})
}
func (u *updatesController) install(w http.ResponseWriter, r *http.Request) {
	var selection updateapproval.Selection
	if err := updateapproval.Decode(http.MaxBytesReader(w, r.Body, 1024), &selection); err != nil || selection.Validate() != nil {
		http.Error(w, "invalid update approval", http.StatusBadRequest)
		return
	}
	u.channelMu.Lock()
	channel, err := updateprefs.Load(u.stateDir)
	if err != nil || channel != selection.Channel {
		u.channelMu.Unlock()
		http.Error(w, "select and save the desired update channel first", http.StatusConflict)
		return
	}
	policy := updatetrust.InspectInstalled(u.runningVersion, u.versionFile).Eligibility(selection.Version)
	if !policy.Eligible {
		u.approvals.Invalidate(u.admins.SessionBinding(r))
		u.channelMu.Unlock()
		http.Error(w, policy.Reason, http.StatusConflict)
		return
	}
	job, err := u.approvals.Consume(u.admins.SessionBinding(r), selection)
	u.channelMu.Unlock()
	if err != nil {
		http.Error(w, "approval expired, consumed or changed; check releases again", http.StatusConflict)
		return
	}
	data, _ := json.Marshal(job)
	// The token stays consumed on IPC failure/409: an uncertain attempt must not
	// silently regain authorization. Fresh discovery is the only retry path.
	u.enqueue(w, r, data)
}

func fetchReleaseResource(ctx context.Context, client *http.Client, url string, max int64, expected int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LocasOS-update-check")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release resource returned HTTP %d", res.StatusCode)
	}
	if res.ContentLength > max || (expected >= 0 && res.ContentLength >= 0 && res.ContentLength != expected) {
		return nil, errors.New("invalid release resource length")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max || (expected >= 0 && int64(len(data)) != expected) {
		return nil, errors.New("invalid release resource size")
	}
	return data, nil
}

func packageFiles(rel releaseInfo) (map[string]string, map[string]releaseAsset, error) {
	files := map[string]string{}
	assets := map[string]releaseAsset{}
	for _, a := range rel.Assets {
		switch a.Name {
		case "locasos-upgrade-amd64.tar", "locasos-upgrade-amd64.tar.sha256", "locasos-update.json", "locasos-update.sig", "Upgrade-LocasOS.ps1", "upgrade-locas.sh":
			if _, duplicate := assets[a.Name]; duplicate {
				return nil, nil, errors.New("duplicate release asset")
			}
			if a.URL != releaseDownloadRoot+rel.Tag+"/"+a.Name || a.Size <= 0 || a.Size > 512*1024*1024 {
				return nil, nil, errors.New("untrusted release asset")
			}
			assets[a.Name] = a
			files[a.Name] = a.URL
		}
	}
	return files, assets, nil
}
func (u *updatesController) signedManifest(ctx context.Context, rel releaseInfo, assets map[string]releaseAsset) (updatetrust.Manifest, error) {
	key, err := base64.StdEncoding.DecodeString(u.publicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return updatetrust.Manifest{}, errors.New("trusted publisher key unavailable; dashboard installation blocked")
	}
	manifestAsset := assets["locasos-update.json"]
	sigAsset := assets["locasos-update.sig"]
	if manifestAsset.Size <= 0 || manifestAsset.Size > 4096 || sigAsset.Size != ed25519.SignatureSize {
		return updatetrust.Manifest{}, errors.New("invalid signed metadata sizes")
	}
	payload, err := fetchReleaseResource(ctx, u.client, manifestAsset.URL, 4096, manifestAsset.Size)
	if err != nil {
		return updatetrust.Manifest{}, err
	}
	sig, err := fetchReleaseResource(ctx, u.client, sigAsset.URL, ed25519.SignatureSize, sigAsset.Size)
	if err != nil {
		return updatetrust.Manifest{}, err
	}
	m, err := updatetrust.Verify(payload, sig, ed25519.PublicKey(key))
	if err != nil {
		return m, err
	}
	if m.Version != rel.Tag {
		return m, errors.New("signed manifest version differs from selected release")
	}
	return m, nil
}
func (u *updatesController) check(w http.ResponseWriter, r *http.Request) {
	session := u.admins.SessionBinding(r)
	u.channelMu.Lock()
	channel := r.URL.Query().Get("channel")
	if channel == "" {
		channel, _ = updateprefs.Load(u.stateDir)
	}
	if !updateprefs.ValidChannel(channel) {
		u.channelMu.Unlock()
		http.Error(w, "unsupported update channel", http.StatusBadRequest)
		return
	}
	generation, err := u.approvals.Begin(session)
	u.channelMu.Unlock()
	if err != nil {
		http.Error(w, "update approval capacity unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	data, err := fetchReleaseResource(ctx, u.client, releasesAPI, 1024*1024, -1)
	if err != nil {
		http.Error(w, "release discovery unavailable", http.StatusServiceUnavailable)
		return
	}
	var releases []releaseInfo
	if err := json.Unmarshal(data, &releases); err != nil {
		http.Error(w, "invalid release metadata", http.StatusBadGateway)
		return
	}
	candidates := make([]updatetrust.ReleaseCandidate, 0, len(releases))
	for _, rel := range releases {
		candidates = append(candidates, updatetrust.ReleaseCandidate{Tag: rel.Tag, Draft: rel.Draft, Prerelease: rel.Prerelease})
	}
	selected, found := updatetrust.SelectLatest(candidates, channel)
	if !found {
		http.Error(w, "no releases available in selected channel", http.StatusNotFound)
		return
	}
	var latest releaseInfo
	matches := 0
	for _, rel := range releases {
		if rel.Tag == selected.Tag {
			latest = rel
			matches++
		}
	}
	if matches != 1 || latest.URL != releasePageRoot+latest.Tag || latest.Draft || (channel == "stable" && latest.Prerelease) {
		http.Error(w, "invalid release identity", http.StatusBadGateway)
		return
	}
	files, assets, err := packageFiles(latest)
	if err != nil {
		http.Error(w, "invalid release asset metadata", http.StatusBadGateway)
		return
	}
	ready := len(files) == 6
	result := map[string]any{
		"channel":            channel,
		"latest_release_tag": latest.Tag,
		"release_url":        latest.URL,
		"published_at":       latest.Published,
		"package_ready":      ready,
		"package_files":      files,
		"install_ready":      false,
		"message":            "Installation requires verified signed metadata and a fresh one-time artifact approval.",
	}
	u.eligibility(result, latest.Tag)
	if ready {
		manifest, err := u.signedManifest(ctx, latest, assets)
		if err != nil {
			result["approval_error"] = "Signed release verification unavailable or invalid; installation blocked."
		} else {
			u.channelMu.Lock()
			saved, saveErr := updateprefs.Load(u.stateDir)
			// Revalidate after network I/O: logout, global channel changes or a newer
			// discovery must not mint authorization from a stale response.
			policy := u.eligibility(result, latest.Tag)
			if policy.Eligible && saveErr == nil && saved == channel && u.admins.SessionBinding(r) == session && session != "" {
				selection, expires, issueErr := u.approvals.Issue(session, generation, manifest.Version, channel, strings.ToLower(manifest.SHA256))
				if issueErr == nil {
					result["install_ready"] = true
					result["approval"] = selection
					result["approval_expires_at"] = expires.UTC().Format(time.RFC3339Nano)
				}
			}
			u.channelMu.Unlock()
		}
	}
	sendJSON(w, http.StatusOK, result)
}
