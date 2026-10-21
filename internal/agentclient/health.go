package agentclient

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"
)

type Health struct {
	Status    string `json:"status"`
	Component string `json:"component"`
}

func Check(ctx context.Context, socket string) (Health, error) {
	if socket == "" {
		return Health{}, errors.New("agent socket path is required")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://agent/healthz", nil)
	if err != nil {
		return Health{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Health{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Health{}, errors.New("agent returned non-200 status")
	}
	var h Health
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 4096)).Decode(&h); err != nil {
		return Health{}, err
	}
	if h.Component != "locas-agent" || h.Status != "ok" {
		return Health{}, errors.New("unexpected agent health payload")
	}
	return h, nil
}
