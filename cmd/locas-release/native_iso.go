package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const nativeISOFile = "locasos-native-uefi-vm-experimental.iso"

func signNativeISO(version, commit, dir string) error {
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) ||
		!regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit) {
		return fmt.Errorf("invalid ISO version or commit")
	}
	private, err := base64.StdEncoding.DecodeString(os.Getenv("LOCAS_RELEASE_ED25519_PRIVATE_KEY_B64"))
	if err != nil || len(private) != ed25519.PrivateKeySize {
		return fmt.Errorf("valid native ISO publisher signing key required")
	}
	hash, err := installHash(filepath.Join(dir, nativeISOFile))
	if err != nil {
		return err
	}
	manifest := nativeManifest{Version: version, Commit: commit, Platform: "linux-amd64-native-iso-vm", File: nativeISOFile, SHA256: hash}
	payload, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(filepath.Join(dir, "locasos-native-iso.json"), payload, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "locasos-native-iso.sig"), ed25519.Sign(ed25519.PrivateKey(private), payload), 0644)
}

func verifyNativeISO(version, dir string) error {
	pub, err := base64.StdEncoding.DecodeString(os.Getenv("LOCAS_UPDATE_PUBLIC_KEY_B64"))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("independently trusted publisher public key required")
	}
	data, err := os.ReadFile(filepath.Join(dir, "locasos-native-iso.json"))
	if err != nil {
		return err
	}
	if len(data) > 4096 {
		return fmt.Errorf("oversized ISO manifest")
	}
	signature, err := os.ReadFile(filepath.Join(dir, "locasos-native-iso.sig"))
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), data, signature) {
		return fmt.Errorf("native ISO manifest signature invalid")
	}
	var m nativeManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m.Version != version || m.Platform != "linux-amd64-native-iso-vm" || m.File != nativeISOFile ||
		!regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(m.Commit) ||
		!regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(m.SHA256) {
		return fmt.Errorf("invalid native ISO identity")
	}
	hash, err := installHash(filepath.Join(dir, nativeISOFile))
	if err != nil {
		return err
	}
	if hash != m.SHA256 {
		return fmt.Errorf("native ISO SHA256 mismatch")
	}
	return nil
}
