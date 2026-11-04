package appmanager

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"time"
)

var versionPattern = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,64}$`)

// RuntimeReady is bounded service/account/API readiness for unprivileged UI
// and Core use, not exact PID/FD attribution. Root installers use the stronger
// RuntimeOwnedReady and independent script ownership checks before effects.
func RuntimeReady(ctx context.Context) bool      { return observedReady(ctx, false) }
func RuntimeOwnedReady(ctx context.Context) bool { return observedReady(ctx, true) }
func observedReady(ctx context.Context, exact bool) bool {
	_, port, err := RuntimeAddress()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	first, ok := localUnit(ctx)
	if !ok {
		return false
	}
	owns := serviceAccountSocket
	if exact {
		owns = exactProcessSocket
	}
	if !owns(first, port) {
		return false
	}
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if !runtimeAPIReady(ctx, client) {
		return false
	}
	after, ok := localUnit(ctx)
	return ok && after == first && owns(after, port)
}
func runtimeAPIReady(ctx context.Context, client *http.Client) bool {
	base, configErr := RuntimeEndpoint()
	if configErr != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/version", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1025))
	if err != nil || len(b) > 1024 {
		return false
	}
	var result struct {
		Version string `json:"version"`
	}
	return json.Unmarshal(b, &result) == nil && versionPattern.MatchString(result.Version)
}
