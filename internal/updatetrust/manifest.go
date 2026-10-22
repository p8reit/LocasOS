package updatetrust

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
)

type Manifest struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	File     string `json:"file"`
	SHA256   string `json:"sha256"`
	Commit   string `json:"commit"`
}

var versionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func (m Manifest) Validate() error {
	if !versionPattern.MatchString(m.Version) || !commitPattern.MatchString(m.Commit) {
		return errors.New("invalid release identity")
	}
	if m.Platform != "linux-amd64-wsl2" || m.File != "locasos-upgrade-amd64.tar" {
		return errors.New("unsupported package target")
	}
	decoded, err := hex.DecodeString(m.SHA256)
	if err != nil || len(decoded) != sha256.Size {
		return errors.New("invalid SHA256 digest")
	}
	return nil
}
func Parse(data []byte) (Manifest, error) {
	var m Manifest
	if len(data) > 4096 {
		return m, errors.New("manifest too large")
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	return m, m.Validate()
}

// Verify authenticates the exact bytes served as the manifest, not a re-encoded JSON object.
func Verify(manifest, signature []byte, pub ed25519.PublicKey) (Manifest, error) {
	m, err := Parse(manifest)
	if err != nil {
		return m, err
	}
	if len(pub) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize || !ed25519.Verify(pub, manifest, signature) {
		return m, errors.New("invalid publisher signature")
	}
	return m, nil
}

func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
