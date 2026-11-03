package updateprefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChannelPreferencePersistsAndDefaultsToStable(t *testing.T) {
	dir := t.TempDir()
	channel, err := Load(dir)
	if err != nil || channel != "stable" {
		t.Fatalf("missing preference should default to stable: %q, %v", channel, err)
	}
	if err := Save(dir, "beta"); err != nil {
		t.Fatal(err)
	}
	channel, err = Load(dir)
	if err != nil || channel != "beta" {
		t.Fatalf("Early Access preference not persisted: %q, %v", channel, err)
	}
	info, err := os.Stat(filepath.Join(dir, fileName))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("saved preferences must be private: %v, %v", info, err)
	}
	if err := Save(dir, "stable"); err != nil {
		t.Fatal(err)
	}
	channel, err = Load(dir)
	if err != nil || channel != "stable" {
		t.Fatalf("Stable preference not restored: %q, %v", channel, err)
	}
}

func TestUnsafeChannelsRejectedAndInvalidStateFailsClosed(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "developer"); err == nil {
		t.Fatal("unsupported channel persisted")
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("anything\n"), 0600); err != nil {
		t.Fatal(err)
	}
	channel, err := Load(dir)
	if err == nil || channel != "stable" {
		t.Fatalf("corrupt preference must fail closed to stable: %q, %v", channel, err)
	}
}
