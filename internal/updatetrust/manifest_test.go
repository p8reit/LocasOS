package updatetrust

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
)

func TestSignedManifestAndTampering(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version: "v0.10.0",
		Platform: "linux-amd64-wsl2",
		File: "locasos-upgrade-amd64.tar",
		SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, data)
	if _, err := Verify(data, sig, pub); err != nil {
		t.Fatal(err)
	}
	data[20] ^= 1
	if _, err := Verify(data, sig, pub); err == nil {
		t.Fatal("tampered manifest accepted")
	}
}

func TestRejectPrereleaseSuffixForPromotableBuilds(t *testing.T) {
	m := Manifest{
		Version: "v0.10.0-beta.1",
		Platform: "linux-amd64-wsl2",
		File: "locasos-upgrade-amd64.tar",
		SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("prerelease version accepted as immutable release identity")
	}
}
