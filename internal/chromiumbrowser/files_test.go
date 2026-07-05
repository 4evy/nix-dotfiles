package chromiumbrowser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildkite/shellwords"
)

func TestWriteWrapperParsesAndQuotesFlags(t *testing.T) {
	target := filepath.Join(t.TempDir(), "helium-browser")
	options := &InstallOptions{
		Flags: `--user-data-dir "/tmp/Helium Profile" --name "O'Brien"`,
		extraWrapperFlags: []string{
			"--class=helium-browser",
		},
	}

	if err := writeWrapper(target, "/opt/Helium/helium-wrapper", options); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(
		text,
		"unset DESKTOP_STARTUP_ID STARTUP_NOTIFICATION_ID XDG_ACTIVATION_TOKEN",
	) {
		t.Fatalf("wrapper %q does not clear startup notification tokens", text)
	}
	for _, want := range []string{
		`unset FONTCONFIG_SYSROOT`,
		`export FONTCONFIG_FILE="${FONTCONFIG_FILE:-/etc/fonts/fonts.conf}"`,
		`export FONTCONFIG_PATH="${FONTCONFIG_PATH:-/etc/fonts}"`,
		`export XDG_DATA_DIRS="${XDG_DATA_DIRS:+$XDG_DATA_DIRS:}/usr/local/share:/usr/share"`,
		`append_flags_file "$XDG_CONFIG_HOME/helium-flags.conf"`,
		`"${runtime_flags[@]}" "$@"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("wrapper %q does not contain %q", text, want)
		}
	}
	for _, want := range []string{
		shellwords.QuotePosix("/opt/Helium/helium-wrapper"),
		"--user-data-dir",
		shellwords.QuotePosix("/tmp/Helium Profile"),
		"--name",
		shellwords.QuotePosix("O'Brien"),
		shellwords.QuotePosix("--class=helium-browser"),
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("wrapper %q does not contain %q", text, want)
		}
	}
}

func TestLinuxDesktopEntryAddsStartupWMClass(t *testing.T) {
	input := strings.Join([]string{
		"[Desktop Entry]",
		"Name=Helium",
		"Exec=helium %U",
		"Actions=new-window;new-private-window;",
		"",
		"[Desktop Action new-window]",
		"Exec=helium",
		"",
		"[Desktop Action new-private-window]",
		"Exec=helium --incognito",
		"",
	}, "\n")

	got := LinuxDesktopEntry(input, "/home/user/.local/bin/helium-browser", "helium", "helium-browser")

	for _, want := range []string{
		"Exec=/home/user/.local/bin/helium-browser %U",
		"StartupNotify=false",
		"StartupWMClass=helium-browser\n[Desktop Action new-window]",
		"Exec=/home/user/.local/bin/helium-browser\n",
		"Exec=/home/user/.local/bin/helium-browser --incognito",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("desktop entry %q does not contain %q", got, want)
		}
	}
}
