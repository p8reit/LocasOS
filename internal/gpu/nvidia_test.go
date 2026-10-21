package gpu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCSV(t *testing.T) {
	d, e := ParseCSV("NVIDIA GeForce RTX 3060, 12288, 4096, 25\n")
	if e != nil || len(d) != 1 || d[0].MemoryUsedMiB != 4096 {
		t.Fatalf("%v %v", d, e)
	}
}
func TestReject(t *testing.T) {
	for _, s := range []string{"GPU, 100, 200, 5", "GPU, 100, 50, 120", "GPU, ??, 5, 2"} {
		if _, e := ParseCSV(s); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestFindNvidiaSMIOnPath(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "nvidia-smi")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := FindNvidiaSMI()
	if err != nil || got != binary {
		t.Fatalf("got %q, err %v", got, err)
	}
}
