package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

var installPublicKeyBase64 string

type installManifest struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Platform string `json:"platform"`
	File     string `json:"file"`
	SHA256   string `json:"sha256"`
}

func main() {
	if len(os.Args)==5 && os.Args[1]=="sign-native" {
		if err:=nativeSigning(os.Args[2],os.Args[3],os.Args[4]);err!=nil {fatal(err.Error())};fmt.Println("PASS: native VM manifest signed");return
	}
	if len(os.Args)==4 && os.Args[1]=="verify-native" {
		if err:=nativeVerification(os.Args[2],os.Args[3]);err!=nil {fatal(err.Error())};fmt.Println("PASS: signed native VM payload verified");return
	}
	if len(os.Args) == 4 && os.Args[1] == "verify-install" {
		if err := verifyInstall(os.Args[2], os.Args[3]); err != nil {
			fatal(err.Error())
		}
		fmt.Println("PASS: signed install image and checksum verified")
		return
	}
	if len(os.Args) == 5 && os.Args[1] == "sign-install" {
		if err := signInstall(os.Args[2], os.Args[3], os.Args[4]); err != nil {
			fatal(err.Error())
		}
		return
	}
	if len(os.Args) < 2 {
		fatal("usage: locas-release keygen FILE | public-key | sign VERSION COMMIT DIST | verify VERSION DIST")
	}
	if os.Args[1] == "keygen" && len(os.Args) == 3 {
		_, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fatal(err.Error())
		}
		file, err := os.OpenFile(os.Args[2], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			fatal(err.Error())
		}
		_, writeErr := fmt.Fprintln(file, base64.StdEncoding.EncodeToString(private))
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			fatal("could not write protected release signing key")
		}
		fmt.Println("Release key created. Keep this file private and out of source control.")
		fmt.Println("Public verification key:", base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey)))
		return
	}
	if os.Args[1] == "verify" && len(os.Args) == 4 {
		version, dir := os.Args[2], os.Args[3]
		publicKey, err := base64.StdEncoding.DecodeString(os.Getenv("LOCAS_UPDATE_PUBLIC_KEY_B64"))
		if err != nil || len(publicKey) != ed25519.PublicKeySize {
			fatal("pinned publisher verification key required")
		}
		hash, err := verifyBundle(version, dir, ed25519.PublicKey(publicKey))
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("Verified signed LOCAS release %s (%s)\n", version, hash)
		return
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("LOCAS_RELEASE_ED25519_PRIVATE_KEY_B64"))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		fatal("valid signing secret required; refusing unsigned release")
	}
	if os.Args[1] == "public-key" && len(os.Args) == 2 {
		fmt.Println(base64.StdEncoding.EncodeToString(ed25519.PrivateKey(key).Public().(ed25519.PublicKey)))
		return
	}
	if os.Args[1] != "sign" || len(os.Args) != 5 {
		fatal("usage: locas-release sign VERSION COMMIT DIST | public-key")
	}
	version, commit, dir := os.Args[2], os.Args[3], os.Args[4]
	archive := filepath.Join(dir, "locasos-upgrade-amd64.tar")
	hash, err := updatetrust.FileHash(archive)
	if err != nil {
		fatal(err.Error())
	}
	m := updatetrust.Manifest{
		Version:  version,
		Platform: "linux-amd64-wsl2",
		File:     "locasos-upgrade-amd64.tar",
		SHA256:   hash,
		Commit:   commit,
	}
	if err := m.Validate(); err != nil {
		fatal(err.Error())
	}
	payload, err := json.Marshal(m)
	if err != nil {
		fatal(err.Error())
	}
	payload = append(payload, '\n')
	sig := ed25519.Sign(ed25519.PrivateKey(key), payload)
	if err := os.WriteFile(filepath.Join(dir, "locasos-update.json"), payload, 0644); err != nil {
		fatal(err.Error())
	}
	if err := os.WriteFile(filepath.Join(dir, "locasos-update.sig"), sig, 0644); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Signed immutable LOCAS update %s\n", version)
}

func verifyBundle(version, dir string, publicKey ed25519.PublicKey) (string, error) {
	manifest, err := os.ReadFile(filepath.Join(dir, "locasos-update.json"))
	if err != nil {
		return "", err
	}
	if len(manifest) > 4096 {
		return "", fmt.Errorf("signed manifest too large")
	}
	signature, err := os.ReadFile(filepath.Join(dir, "locasos-update.sig"))
	if err != nil {
		return "", err
	}
	validated, err := updatetrust.Verify(manifest, signature, publicKey)
	if err != nil {
		return "", err
	}
	if validated.Version != version {
		return "", fmt.Errorf("publisher signature valid but release tag mismatches manifest")
	}
	hash, err := updatetrust.FileHash(filepath.Join(dir, validated.File))
	if err != nil {
		return "", err
	}
	if hash != validated.SHA256 {
		return "", fmt.Errorf("publisher signature valid but archive digest differs")
	}
	return hash, nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func installHash(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func signInstall(version, commit, dir string) error {
	private, err := base64.StdEncoding.DecodeString(os.Getenv("LOCAS_RELEASE_ED25519_PRIVATE_KEY_B64"))
	if err != nil || len(private) != ed25519.PrivateKeySize {
		return fmt.Errorf("publisher signing key missing")
	}
	hash, err := installHash(filepath.Join(dir, "locasos-wsl-amd64.tar"))
	if err != nil {
		return err
	}
	m := installManifest{version, commit, "linux-amd64-wsl2", "locasos-wsl-amd64.tar", hash}
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(filepath.Join(dir, "locasos-install.json"), payload, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "locasos-install.sig"), ed25519.Sign(ed25519.PrivateKey(private), payload), 0644)
}

func verifyInstall(version, dir string) error {
	pinned := installPublicKeyBase64
	if pinned == "" {
		pinned = os.Getenv("LOCAS_UPDATE_PUBLIC_KEY_B64")
	}
	key, err := base64.StdEncoding.DecodeString(pinned)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("trusted publisher key not configured")
	}
	data, err := os.ReadFile(filepath.Join(dir, "locasos-install.json"))
	if err != nil {
		return err
	}
	if len(data) > 4096 {
		return fmt.Errorf("install manifest too large")
	}
	sig, err := os.ReadFile(filepath.Join(dir, "locasos-install.sig"))
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(key), data, sig) {
		return fmt.Errorf("install manifest signature invalid")
	}
	var m installManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m.Version != version || m.Platform != "linux-amd64-wsl2" || m.File != "locasos-wsl-amd64.tar" || len(m.Commit) != 40 || len(m.SHA256) != 64 {
		return fmt.Errorf("invalid install image identity")
	}
	hash, err := installHash(filepath.Join(dir, m.File))
	if err != nil {
		return err
	}
	if hash != m.SHA256 {
		return fmt.Errorf("install image SHA256 mismatch")
	}
	return nil
}
