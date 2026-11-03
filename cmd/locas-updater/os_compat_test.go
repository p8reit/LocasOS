package main

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseOSCompatibility(t *testing.T) {
	dir := t.TempDir()
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=debian\nVERSION_ID=\"13\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	writeArchive := func(target string) string {
		t.Helper()
		file := filepath.Join(dir, strings.ReplaceAll(target, ":", "-")+".tar")
		f, err := os.Create(file)
		if err != nil {
			t.Fatal(err)
		}
		w := tar.NewWriter(f)
		b := []byte(target + "\n")
		if err := w.WriteHeader(&tar.Header{Name: "opt/locas/bin/locas-os-base", Mode: 0644, Size: int64(len(b))}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(b); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		return file
	}
	if err := checkReleaseOSBase(writeArchive("debian:13"), osRelease); err != nil {
		t.Fatalf("matching Debian target should pass: %v", err)
	}
	if err := checkReleaseOSBase(writeArchive("ubuntu:24.04"), osRelease); err == nil {
		t.Fatal("cross-distribution release must fail")
	}
	if err := checkReleaseOSBase(writeArchive("debian:12"), osRelease); err == nil {
		t.Fatal("cross-version release must fail")
	}
}
