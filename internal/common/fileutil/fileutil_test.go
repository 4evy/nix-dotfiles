package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTextIfChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file.txt")
	changed, err := WriteTextIfChanged(path, "hello")
	if err != nil || !changed {
		t.Fatalf("first write = %v, %v; want changed", changed, err)
	}
	changed, err = WriteTextIfChanged(path, "hello")
	if err != nil || changed {
		t.Fatalf("second write = %v, %v; want unchanged", changed, err)
	}
}

func TestWriteJSONIfChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file.json")
	value := map[string]string{"hello": "world"}

	changed, err := WriteJSONIfChanged(path, value, PrivateFilePerm)
	if err != nil || !changed {
		t.Fatalf("first write = %v, %v; want changed", changed, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "{\n  \"hello\": \"world\"\n}\n"; got != want {
		t.Fatalf("json = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != PrivateFilePerm {
		t.Fatalf("mode = %#o; want 0600", got)
	}

	changed, err = WriteJSONIfChanged(path, value, PrivateFilePerm)
	if err != nil || changed {
		t.Fatalf("second write = %v, %v; want unchanged", changed, err)
	}
}

func TestMakeExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), PrivateFilePerm); err != nil {
		t.Fatal(err)
	}
	if err := MakeExecutable(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != PrivateFilePerm|ExecutablePerm {
		t.Fatalf("mode = %#o; want 0711", got)
	}
}
