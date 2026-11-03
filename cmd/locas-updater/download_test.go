package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadStreamsAndRejectsIncorrectSizes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		expected int64
		code     int
		ok       bool
	}{
		{name: "complete", body: "abcdef", expected: 6, code: 200, ok: true},
		{name: "short response", body: "abc", expected: 6, code: 200},
		{name: "oversized response", body: "abcdef", expected: 4, code: 200},
		{name: "empty expected size", body: "abcdef", expected: 0, code: 200},
		{name: "server error", body: "no", expected: 2, code: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			directory := t.TempDir()
			destination := filepath.Join(directory, "update.tar")
			client := &http.Client{Timeout: 5 * time.Second}
			err := download(context.Background(), client, server.URL, destination, tc.expected)
			if tc.ok {
				if err != nil {
					t.Fatalf("download failed: %v", err)
				}
				data, err := os.ReadFile(destination)
				if err != nil || string(data) != tc.body {
					t.Fatalf("incorrect output: %q, %v", data, err)
				}
			} else {
				if err == nil {
					t.Fatal("accepted incomplete, oversized or invalid asset")
				}
				if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
					t.Fatalf("unverified asset written to final path: %v", statErr)
				}
			}
			partials, err := filepath.Glob(filepath.Join(directory, ".locas-download-*"))
			if err != nil || len(partials) != 0 {
				t.Fatalf("temporary download files retained: %v, %v", partials, err)
			}
		})
	}
}

func TestFailedDownloadDoesNotReplaceExistingAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("broken"))
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "update.tar")
	if err := os.WriteFile(destination, []byte("old validated asset"), 0600); err != nil {
		t.Fatal(err)
	}
	err := download(context.Background(), server.Client(), server.URL, destination, 100)
	if err == nil {
		t.Fatal("incomplete HTTP response was accepted")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "old validated asset" {
		t.Fatalf("incomplete download overwrote previous asset: %q (%v)", data, err)
	}
}

func TestRefuseAutomaticDowngrades(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "current-version")
	if err := refuseDowngrade(file, "v0.11.0"); err != nil {
		t.Fatalf("legacy versionless installation blocked: %v", err)
	}
	for _, tc := range []struct {
		installed string
		target    string
		allow     bool
	}{
		{installed: "v0.11.0", target: "v0.10.0", allow: false},
		{installed: "v0.11.0", target: "v0.11.0", allow: true},
		{installed: "v0.11.0", target: "v0.12.0", allow: true},
		{installed: "invalid", target: "v0.12.0", allow: false},
	} {
		if err := os.WriteFile(file, []byte(tc.installed+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		err := refuseDowngrade(file, tc.target)
		if (err == nil) != tc.allow {
			t.Fatalf("from %s to %s: result %v; allow=%v", tc.installed, tc.target, err, tc.allow)
		}
		if !tc.allow && err != nil && !strings.Contains(err.Error(), "downgrade") &&
			!strings.Contains(err.Error(), "invalid installed") {
			t.Fatalf("unexpected block reason: %v", err)
		}
	}
}
