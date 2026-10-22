package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

func main() {
	if len(os.Args) != 5 || os.Args[1] != "sign" {
		fatal("usage: locas-release sign VERSION COMMIT DIST")
	}
	version, commit, dir := os.Args[2], os.Args[3], os.Args[4]
	key, err := base64.StdEncoding.DecodeString(os.Getenv("LOCAS_RELEASE_ED25519_PRIVATE_KEY_B64"))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		fatal("valid signing secret required; refusing unsigned release")
	}
	archive := filepath.Join(dir, "locasos-upgrade-amd64.tar")
	hash, err := updatetrust.FileHash(archive)
	if err != nil {
		fatal(err.Error())
	}
	m := updatetrust.Manifest{
		Version: version,
		Platform: "linux-amd64-wsl2",
		File: "locasos-upgrade-amd64.tar",
		SHA256: hash,
		Commit: commit,
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
