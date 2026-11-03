package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

func readBounded(filename string, max int64) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errors.New("release metadata exceeds size limit")
	}
	return data, nil
}

// verifyBundle uses a separately provisioned publisher key, never a release asset.
// It authenticates original manifest bytes and the exact archive before promotion.
func verifyApprovedBundle(version, commit, dir string, key ed25519.PublicKey) error {
	manifest, err := readBounded(filepath.Join(dir, "locasos-update.json"), 4096)
	if err != nil {
		return err
	}
	sig, err := readBounded(filepath.Join(dir, "locasos-update.sig"), ed25519.SignatureSize)
	if err != nil {
		return err
	}
	m, err := updatetrust.Verify(manifest, sig, key)
	if err != nil {
		return err
	}
	if m.Version != version || m.Commit != commit {
		return errors.New("signed identity differs from approved tag and commit")
	}
	archive := filepath.Join(dir, m.File)
	info, err := os.Stat(archive)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 512*1024*1024 {
		return errors.New("invalid release archive size or type")
	}
	digest, err := updatetrust.FileHash(archive)
	if err != nil {
		return err
	}
	if !strings.EqualFold(digest, m.SHA256) {
		return errors.New("release archive digest mismatch")
	}
	fmt.Printf("Verified signed release %s at %s, archive SHA-256 %s\n", version, commit, digest)
	return nil
}
