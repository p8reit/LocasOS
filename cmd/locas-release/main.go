package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

func main() {
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
		manifest, err := os.ReadFile(filepath.Join(dir, "locasos-update.json"))
		if err != nil {
			fatal(err.Error())
		}
		if len(manifest) > 4096 {
			fatal("signed manifest too large")
		}
		signature, err := os.ReadFile(filepath.Join(dir, "locasos-update.sig"))
		if err != nil {
			fatal(err.Error())
		}
		validated, err := updatetrust.Verify(manifest, signature, ed25519.PublicKey(publicKey))
		if err != nil {
			fatal(err.Error())
		}
		if validated.Version != version {
			fatal("publisher signature valid but release tag mismatches manifest")
		}
		hash, err := updatetrust.FileHash(filepath.Join(dir, validated.File))
		if err != nil {
			fatal(err.Error())
		}
		if hash != validated.SHA256 {
			fatal("publisher signature valid but archive digest differs")
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

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
