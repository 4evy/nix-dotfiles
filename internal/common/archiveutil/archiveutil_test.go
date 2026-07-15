package archiveutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeLocalPathRejectsEscapingArchivePath(t *testing.T) {
	for _, name := range []string{"../file.txt", `..\file.txt`, "root/../../file.txt", "/tmp/file.txt"} {
		t.Run(name, func(t *testing.T) {
			_, err := safeLocalPath(name)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "escapes destination") {
				t.Fatalf("error = %q", err)
			}
		})
	}
}

func TestExtractEntryRejectsSymlinkEscape(t *testing.T) {
	rootDir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(rootDir, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close root: %v", err)
		}
	})

	err = extractEntry(root, "link/escape.txt", 0o644, strings.NewReader("owned"))
	if err == nil {
		t.Fatal("expected symlink escape to fail")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "escape.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("outside file stat err = %v; want not exist", statErr)
	}
}
