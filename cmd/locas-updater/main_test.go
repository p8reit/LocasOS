package main

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"
)

func archiveForTest(t *testing.T, items []tar.Header) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "update.tar")
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	for _, item := range items {
		if err := writer.WriteHeader(&item); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestArchiveAllowlist(t *testing.T) {
	valid := []tar.Header{
		{Name: "opt/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "opt/locas/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "opt/locas/bin/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "opt/locas/bin/locas-core", Typeflag: tar.TypeReg, Mode: 0755},
	}
	if err := validateArchive(archiveForTest(t, valid)); err != nil {
		t.Fatal(err)
	}
	for _, malicious := range []tar.Header{
		{Name: "./", Typeflag: tar.TypeDir, Mode: 0700},
		{Name: "../etc/passwd", Typeflag: tar.TypeReg},
		{Name: "var/lib/locas-core/passwords", Typeflag: tar.TypeReg},
		{Name: "opt/locas/bin/rogue", Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"},
	} {
		items := append(append([]tar.Header{}, valid...), malicious)
		if err := validateArchive(archiveForTest(t, items)); err == nil {
			t.Fatalf("unsafe archive entry accepted: %s", malicious.Name)
		}
	}
}

func TestApprovedReleaseChannelAfterPromotion(t *testing.T) {
	for _, tc := range []struct {
		name       string
		channel    string
		prerelease bool
		draft      bool
		tag        string
		want       bool
	}{
		{"beta initially prerelease", "beta", true, false, "v0.10.0", true},
		{"beta promoted stable", "beta", false, false, "v0.10.0", true},
		{"stable published", "stable", false, false, "v0.10.0", true},
		{"stable cannot install prerelease", "stable", true, false, "v0.10.0", false},
		{"draft forbidden", "beta", true, true, "v0.10.0", false},
		{"other tag forbidden", "beta", true, false, "v0.10.1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := releaseAllowed(release{Tag: tc.tag, Draft: tc.draft, Prerelease: tc.prerelease},
				request{Channel: tc.channel, Version: "v0.10.0"})
			if got != tc.want {
				t.Fatalf("releaseAllowed=%v, want %v", got, tc.want)
			}
		})
	}
}
