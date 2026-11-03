package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

func TestVerifyApprovedBundle(t *testing.T) {
	for _, scenario := range []string{"valid", "tampered manifest", "tampered archive", "wrong key", "wrong version", "wrong commit", "oversized manifest", "oversized signature", "missing archive"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			pub, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "locasos-upgrade-amd64.tar")
			write := func(name string, data []byte) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("locasos-upgrade-amd64.tar", []byte("exact archive bytes"))
			digest, err := updatetrust.FileHash(archive)
			if err != nil {
				t.Fatal(err)
			}
			commit := strings.Repeat("a", 40)
			version := "v1.2.3"
			m := updatetrust.Manifest{Version: version, Commit: commit, Platform: "linux-amd64-wsl2", File: "locasos-upgrade-amd64.tar", SHA256: digest}
			payload, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			sig := ed25519.Sign(private, payload)
			switch scenario {
			case "tampered manifest":
				payload = append(payload, '\n')
			case "tampered archive":
				write("locasos-upgrade-amd64.tar", []byte("changed"))
			case "wrong key":
				pub, _, err = ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
			case "wrong version":
				version = "v1.2.4"
			case "wrong commit":
				commit = strings.Repeat("b", 40)
			case "oversized manifest":
				payload = make([]byte, 4097)
			case "oversized signature":
				sig = append(sig, 0)
			case "missing archive":
				if err := os.Remove(archive); err != nil {
					t.Fatal(err)
				}
			}
			write("locasos-update.json", payload)
			write("locasos-update.sig", sig)
			err = verifyApprovedBundle(version, commit, dir, pub)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
}
