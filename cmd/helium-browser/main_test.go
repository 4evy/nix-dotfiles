package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/4evy/dotfiles/internal/common/fileutil"
)

func TestReadApplyInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(`{
		"cookie_allowlist": ["[*.]example.com"],
		"extension_values": {"access-token": "secret"}
	}`), fileutil.PrivateFilePerm); err != nil {
		t.Fatal(err)
	}
	input, err := readApplyInput(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.CookieAllowlist) != 1 || input.CookieAllowlist[0] != "[*.]example.com" {
		t.Fatalf("cookie allowlist = %q", input.CookieAllowlist)
	}
	if got := input.ExtensionValues["access-token"]; got != "secret" {
		t.Fatalf("extension input = %q", got)
	}
}

func TestReadApplyInputRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(
		path,
		[]byte(`{"unknown": true}`),
		fileutil.PrivateFilePerm,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := readApplyInput(path); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}
