package codex

import (
	"strconv"

	"github.com/4evy/dotfiles/internal/theme/terminal"
)

const (
	configFlag     = "-c"
	themeConfigKey = "tui.theme="
	darkThemeName  = "catppuccin-frappe-pink"
	lightThemeName = "catppuccin-latte-pink"
)

// ThemeArgs prepends the terminal theme selected for the Codex TUI.
func ThemeArgs(mode terminal.Mode, extraArgs []string) []string {
	name := darkThemeName
	if mode == terminal.Light {
		name = lightThemeName
	}
	args := []string{configFlag, themeConfigKey + strconv.Quote(name)}
	return append(args, extraArgs...)
}
