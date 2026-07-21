package terminal

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

type Mode int

const (
	Dark Mode = iota
	Light
)

const (
	terminalThemeProbeTimeout = 100 * time.Millisecond

	colorSchemeDarkReport  = "\x1b[?997;1n"
	colorSchemeLightReport = "\x1b[?997;2n"
	oscBackgroundCommand   = 11
	oscBackgroundPrefix    = "11;"
	oscRGBPrefix           = "rgb:"

	hexRadix                   = 16
	bitsPerHexDigit            = 4
	maxColorComponentBits      = 16
	maxColorComponentHexDigits = maxColorComponentBits / bitsPerHexDigit

	perceivedRedWeight   = 0.299
	perceivedGreenWeight = 0.587
	perceivedBlueWeight  = 0.114
	lightThemeThreshold  = 0.5
)

var themeEnvironmentVariables = []string{"COLOR_SCHEME", "TERMINAL_THEME", "THEME"}

type themeProbePlan struct {
	fallback Mode
	commands [][]string
}

var unixThemeProbeCommands = [][]string{
	{"gsettings", "get", "org.gnome.desktop.interface", "color-scheme"},
	{"gsettings", "get", "org.gnome.desktop.interface", "gtk-theme"},
}

var systemThemeProbePlans = map[string]themeProbePlan{
	"darwin": {
		fallback: Light,
		commands: [][]string{
			{"defaults", "read", "-g", "AppleInterfaceStyle"},
		},
	},
	"linux":     {fallback: Dark, commands: unixThemeProbeCommands},
	"freebsd":   {fallback: Dark, commands: unixThemeProbeCommands},
	"dragonfly": {fallback: Dark, commands: unixThemeProbeCommands},
	"netbsd":    {fallback: Dark, commands: unixThemeProbeCommands},
	"openbsd":   {fallback: Dark, commands: unixThemeProbeCommands},
}

func Detect() Mode {
	for _, name := range themeEnvironmentVariables {
		if mode, ok := themeModeFromText(os.Getenv(name)); ok {
			return mode
		}
	}
	if mode, ok := detectTerminalTheme(terminalThemeProbeTimeout); ok {
		return mode
	}
	plan, ok := systemThemeProbePlans[runtime.GOOS]
	if !ok {
		return Dark
	}
	for _, command := range plan.commands {
		output, err := exec.Command(command[0], command[1:]...).Output()
		if err == nil {
			if mode, ok := themeModeFromText(string(output)); ok {
				return mode
			}
		}
	}
	return plan.fallback
}

func themeModeFromText(text string) (Mode, bool) {
	text = strings.ToLower(strings.TrimSpace(text))
	if strings.Contains(text, "dark") {
		return Dark, true
	}
	if strings.Contains(text, "light") {
		return Light, true
	}
	switch strings.Trim(text, "'\"") {
	case "frappe", "macchiato", "mocha", "catppuccin-frappe-pink":
		return Dark, true
	case "latte", "catppuccin-latte-pink":
		return Light, true
	}
	return Dark, false
}

func ParseReport(buffer []byte) (Mode, bool) {
	if bytes.Contains(buffer, []byte(colorSchemeDarkReport)) {
		return Dark, true
	}
	if bytes.Contains(buffer, []byte(colorSchemeLightReport)) {
		return Light, true
	}

	parser := ansi.NewParser()
	state := byte(0)
	for len(buffer) > 0 {
		_, _, n, nextState := ansi.DecodeSequence(buffer, state, parser)
		if n <= 0 {
			return Dark, false
		}
		if parser.Command() == oscBackgroundCommand {
			if mode, ok := themeModeFromOSC11Data(parser.Data()); ok {
				return mode, true
			}
		}
		state = nextState
		buffer = buffer[n:]
	}
	return Dark, false
}

func themeModeFromOSC11Data(data []byte) (Mode, bool) {
	value, ok := strings.CutPrefix(string(data), oscBackgroundPrefix)
	if !ok {
		return Dark, false
	}
	value = strings.TrimPrefix(value, oscRGBPrefix)
	red, rest, ok := strings.Cut(value, "/")
	if !ok {
		return Dark, false
	}
	green, blue, ok := strings.Cut(rest, "/")
	if !ok || strings.Contains(blue, "/") {
		return Dark, false
	}
	r, ok := parseColorComponent(red)
	if !ok {
		return Dark, false
	}
	g, ok := parseColorComponent(green)
	if !ok {
		return Dark, false
	}
	b, ok := parseColorComponent(blue)
	if !ok {
		return Dark, false
	}
	// Match Ghostty's light/dark background classification. Relative WCAG
	// luminance is useful for contrast, but its linearized components classify
	// common midtone backgrounds differently from the terminal itself.
	luminance := perceivedRedWeight*r + perceivedGreenWeight*g + perceivedBlueWeight*b
	if luminance > lightThemeThreshold {
		return Light, true
	}
	return Dark, true
}

func parseColorComponent(value string) (float64, bool) {
	if value == "" || len(value) > maxColorComponentHexDigits {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, hexRadix, maxColorComponentBits)
	if err != nil {
		return 0, false
	}
	maxValue := uint64(1<<(len(value)*bitsPerHexDigit)) - 1
	return float64(parsed) / float64(maxValue), true
}
