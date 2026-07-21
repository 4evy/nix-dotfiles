package codex

import (
	"slices"
	"testing"

	"github.com/4evy/dotfiles/internal/theme/terminal"
)

func TestThemeArgs(t *testing.T) {
	extraArgs := []string{"resume", "--last"}
	got := ThemeArgs(terminal.Dark, extraArgs)
	want := []string{
		"-c", `tui.theme="catppuccin-frappe-pink"`,
		"resume", "--last",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ThemeArgs() = %#v, want %#v", got, want)
	}
}
