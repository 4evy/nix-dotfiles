package btop

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	"github.com/4evy/dotfiles/internal/theme/terminal"
)

func TestThemeArgsReplacesConfigWithThemedCopy(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "cache")
	config := filepath.Join(home, "btop.conf")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", cache)
	if err := os.WriteFile(
		config,
		[]byte("foo = true\ncolor_theme = \"old\"\n"),
		fileutil.PrivateFilePerm,
	); err != nil {
		t.Fatal(err)
	}

	args, cleanup, err := ThemeArgs(terminal.Light, []string{longConfigArg, config, "--utf-force"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(args) != 3 || args[0] != longConfigArg || args[2] != "--utf-force" {
		t.Fatalf("ThemeArgs() = %#v", args)
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		t.Fatal(err)
	}
	if text := string(data); !strings.Contains(text, `color_theme = "catppuccin_latte_pink"`) || strings.Contains(text, "old") {
		t.Fatalf("themed config = %q", text)
	}
	if !slices.Equal(
		stripConfigArgs([]string{shortConfigArg + "=one", longConfigArg, "two", "x"}),
		[]string{"x"},
	) {
		t.Fatal("config arguments were not stripped")
	}
}
