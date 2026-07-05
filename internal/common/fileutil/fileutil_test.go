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

	changed, err := WriteJSONIfChanged(path, value, 0o600)
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
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %#o; want 0600", got)
	}

	changed, err = WriteJSONIfChanged(path, value, 0o600)
	if err != nil || changed {
		t.Fatalf("second write = %v, %v; want unchanged", changed, err)
	}
}

func TestCopyAndMoveDirRecursive(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	moved := filepath.Join(root, "moved")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(src, "nested", "leaf.txt"),
		[]byte("leaf"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := CopyDirRecursive(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(
		filepath.Join(dst, "nested", "leaf.txt"),
	); err != nil ||
		string(got) != "leaf" {
		t.Fatalf("copied file = %q, %v", got, err)
	}
	if err := MoveDir(dst, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("old dir stat err = %v; want not exist", err)
	}
}

func TestMakeExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MakeExecutable(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o711 {
		t.Fatalf("mode = %#o; want 0711", got)
	}
}

func TestRelativeUnder(t *testing.T) {
	root := t.TempDir()
	if !RelativeUnder(root, filepath.Join(root, "child")) {
		t.Fatal("child should be under root")
	}
	if RelativeUnder(root, root) {
		t.Fatal("root itself should not count as under root")
	}
	if RelativeUnder(root, filepath.Dir(root)) {
		t.Fatal("parent should not be under root")
	}
}
