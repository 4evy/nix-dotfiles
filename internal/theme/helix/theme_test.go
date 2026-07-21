package helix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	"github.com/4evy/dotfiles/internal/theme/terminal"
)

func TestThemeArgsReplacesConfigWithThemedCopy(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "config.toml")
	t.Setenv("HOME", home)
	if err := os.WriteFile(
		config,
		[]byte("theme = \"old\"\n[editor]\nline-number = \"relative\"\n"),
		fileutil.PrivateFilePerm,
	); err != nil {
		t.Fatal(err)
	}

	args, cleanup, err := ThemeArgs(terminal.Dark, []string{"-c=" + config, "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(args) != 3 || args[0] != longConfigArg || args[2] != "README.md" {
		t.Fatalf("ThemeArgs() = %#v", args)
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `theme = 'catppuccin_frappe_pink'`) ||
		!strings.Contains(text, `line-number = 'relative'`) || strings.Contains(text, "old") {
		t.Fatalf("themed config = %q", text)
	}
}
