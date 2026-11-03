package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

func TestVerifyBundleTrustBoundary(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := []byte("sample-release-archive")
	digest := sha256.Sum256(archive)
	manifest := updatetrust.Manifest{
		Version: "v0.10.0", Platform: "linux-amd64-wsl2",
		File: "locasos-upgrade-amd64.tar", SHA256: hex.EncodeToString(digest[:]),
		Commit: "0123456789abcdef0123456789abcdef01234567",
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(manifest.File, archive)
	write("locasos-update.json", raw)
	write("locasos-update.sig", ed25519.Sign(private, raw))
	if _, err := verifyBundle(manifest.Version, dir, public); err != nil {
		t.Fatalf("valid release rejected: %v", err)
	}
	if _, err := verifyBundle("v0.11.0", dir, public); err == nil {
		t.Fatal("different release tag accepted")
	}
	wrong, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyBundle(manifest.Version, dir, wrong); err == nil {
		t.Fatal("wrong pinned public key accepted")
	}
	write(manifest.File, []byte("modified archive"))
	if _, err := verifyBundle(manifest.Version, dir, public); err == nil {
		t.Fatal("tampered archive accepted")
	}
	write(manifest.File, archive)
	write("locasos-update.json", append([]byte(nil), append(raw, byte(' '))...))
	if _, err := verifyBundle(manifest.Version, dir, public); err == nil {
		t.Fatal("tampered signed manifest accepted")
	}
	write("locasos-update.json", raw)
	write("locasos-update.sig", []byte("forgery"))
	if _, err := verifyBundle(manifest.Version, dir, public); err == nil {
		t.Fatal("forged signature accepted")
	}
}
