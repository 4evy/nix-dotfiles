package terminal

import (
	"math"
	"testing"
)

func TestTerminalThemeReport(t *testing.T) {
	tests := []struct {
		name   string
		report string
		want   Mode
	}{
		{name: "color scheme dark", report: "\x1b[?997;1n", want: Dark},
		{name: "color scheme light", report: "\x1b[?997;2n", want: Light},
		{name: "OSC 11 16-bit light", report: "\x1b]11;rgb:efff/f1f1/f5f5\a", want: Light},
		{name: "OSC 11 16-bit dark with ST", report: "prefix\x1b]11;rgb:3030/3434/4646\x1b\\suffix", want: Dark},
		{name: "OSC 11 8-bit light", report: "\x1b]11;rgb:ef/f1/f5\a", want: Light},
		{name: "OSC 11 8-bit dark", report: "\x1b]11;rgb:30/34/46\a", want: Dark},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := ParseReport([]byte(test.report)); !ok || got != test.want {
				t.Fatalf("ParseReport() = %v, %v, want %v, true", got, ok, test.want)
			}
		})
	}
}

func TestTerminalThemeQuery(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "ghostty")
	if got := terminalThemeQuery(); got != colorSchemeQuery {
		t.Fatalf("Ghostty query = %q, want %q", got, colorSchemeQuery)
	}

	t.Setenv("TERM_PROGRAM", "tmux")
	if got := terminalThemeQuery(); got != backgroundQuery {
		t.Fatalf("generic terminal query = %q, want %q", got, backgroundQuery)
	}
}

func TestOSC11UsesGhosttyPerceivedLuminance(t *testing.T) {
	if got, ok := themeModeFromOSC11Data([]byte("11;rgb:80/80/80")); !ok || got != Light {
		t.Fatalf("#808080 = %v, %v, want light", got, ok)
	}
	if got, ok := themeModeFromOSC11Data([]byte("11;rgb:7f/7f/7f")); !ok || got != Dark {
		t.Fatalf("#7f7f7f = %v, %v, want dark", got, ok)
	}
}

func TestParseColorComponentScalesComponentWidths(t *testing.T) {
	for _, value := range []string{"f", "ff", "fff", "ffff"} {
		if got, ok := parseColorComponent(value); !ok || math.Abs(got-1) > 1e-12 {
			t.Fatalf("parseColorComponent(%q) = %v, %v, want 1, true", value, got, ok)
		}
	}
	for _, value := range []string{"", "fffff", "gg"} {
		if got, ok := parseColorComponent(value); ok {
			t.Fatalf("parseColorComponent(%q) = %v, true, want invalid", value, got)
		}
	}
}

func TestThemeModeFromGnomeDefaults(t *testing.T) {
	for _, text := range []string{"'default'", "'Adwaita'"} {
		if got, ok := themeModeFromText(text); ok {
			t.Fatalf("%q = %v, want no detected mode", text, got)
		}
	}
	if got, ok := themeModeFromText("'Adwaita-dark'"); !ok || got != Dark {
		t.Fatalf("Adwaita-dark = %v, %v, want dark", got, ok)
	}
}

func TestThemeModeFromCatppuccinNames(t *testing.T) {
	if got, ok := themeModeFromText("catppuccin-frappe-pink"); !ok || got != Dark {
		t.Fatalf("frappe = %v, %v, want dark", got, ok)
	}
	if got, ok := themeModeFromText("catppuccin-latte-pink"); !ok || got != Light {
		t.Fatalf("latte = %v, %v, want light", got, ok)
	}
}
