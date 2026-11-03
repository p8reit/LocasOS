package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/p8reit/LocasOS/internal/updateapproval"
	"github.com/p8reit/LocasOS/internal/updatetrust"
)

func TestWorkerRejectsArtifactSubstitutionBeforeApply(t *testing.T) {
	for _, scenario := range []string{"approved", "replacement signed artifact", "archive substitution", "tampered manifest", "wrong key", "wrong version", "missing digest", "missing token"} {
		t.Run(scenario, func(t *testing.T) {
			pub, key, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte("original approved archive")
			sum := sha256.Sum256(payload)
			approvedDigest := hex.EncodeToString(sum[:])
			job := request{Selection: updateapproval.Selection{Version: "v1.2.3", Channel: "stable", SHA256: approvedDigest, Token: strings.Repeat("c", 64)}, ExpiresAt: time.Now().Add(time.Minute)}
			manifest := updatetrust.Manifest{Version: job.Version, Platform: "linux-amd64-wsl2", File: "locasos-upgrade-amd64.tar", SHA256: approvedDigest, Commit: strings.Repeat("a", 40)}
			switch scenario {
			case "replacement signed artifact":
				payload = []byte("different validly signed archive")
				sum = sha256.Sum256(payload)
				manifest.SHA256 = hex.EncodeToString(sum[:])
			case "archive substitution":
				payload = []byte("corrupted download")
			case "wrong key":
				pub, _, err = ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
			case "wrong version":
				manifest.Version = "v1.2.4"
			case "missing digest":
				job.SHA256 = ""
			case "missing token":
				job.Token = ""
			}
			data, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			sig := ed25519.Sign(key, data)
			if scenario == "tampered manifest" {
				data = append(data, '\n')
			}
			archive := filepath.Join(t.TempDir(), "archive.tar")
			if err := os.WriteFile(archive, payload, 0600); err != nil {
				t.Fatal(err)
			}
			digest, err := verifyApprovedArtifact(data, sig, pub, job, archive)
			if scenario == "approved" {
				if err != nil || digest != approvedDigest {
					t.Fatalf("valid approval failed: %v", err)
				}
			} else if err == nil || digest != "" {
				t.Fatalf("unapproved bytes authorized: %v", err)
			}
		})
	}
}

func TestQueuedApprovalReadIsBoundedAndRegular(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid")
	if err := os.WriteFile(valid, []byte(`{"version":"v1.2.3"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := readQueuedApproval(valid); err != nil || string(data) != `{"version":"v1.2.3"}` {
		t.Fatalf("regular read: %s %v", data, err)
	}
	oversized := filepath.Join(dir, "oversized")
	if err := os.WriteFile(oversized, make([]byte, 1025), 0600); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(dir, "linked")
	if err := os.Symlink(valid, linked); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{oversized, linked, fifo, dir} {
		if _, err := readQueuedApproval(file); err == nil {
			t.Fatalf("unsafe approval file accepted: %s", file)
		}
	}
}
