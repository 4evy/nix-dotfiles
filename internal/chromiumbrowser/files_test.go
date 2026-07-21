package chromiumbrowser

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWriteWrapperQuotesArguments(t *testing.T) {
	target := filepath.Join(t.TempDir(), "helium-browser")
	options := &InstallOptions{
		Flags: []string{"--user-data-dir", "/tmp/Helium Profile", "--name", "O'Brien"},
		extraWrapperFlags: []string{
			"--class=helium-browser",
		},
	}

	if err := writeWrapper(
		target,
		"/opt/Helium/helium-wrapper",
		"/tmp/config/helium-flags.conf",
		options,
	); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(
		text,
		`"DESKTOP_STARTUP_ID",`,
	) {
		t.Fatalf("wrapper %q does not clear startup notification tokens", text)
	}
	for _, want := range []string{
		`"FONTCONFIG_SYSROOT",`,
		`os.environ.setdefault("FONTCONFIG_FILE", "/etc/fonts/fonts.conf")`,
		`os.environ.setdefault("FONTCONFIG_PATH", "/etc/fonts")`,
		`os.environ["XDG_DATA_DIRS"]`,
		`FLAGS_FILE: str = "/tmp/config/helium-flags.conf"`,
		`flags_file = Path(FLAGS_FILE) if FLAGS_FILE else None`,
		`config_home = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config"))`,
		`*flags, *sys.argv[1:]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("wrapper %q does not contain %q", text, want)
		}
	}
	for _, want := range []string{
		`/opt/Helium/helium-wrapper`,
		"--user-data-dir",
		`/tmp/Helium Profile`,
		"--name",
		`O'Brien`,
		`--class=helium-browser`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("wrapper %q does not contain %q", text, want)
		}
	}
}

func TestWrapperRuntimeCombinesFlagsAndSanitizesEnvironment(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required by the generated browser wrapper")
	}
	dir := t.TempDir()
	launcher := filepath.Join(dir, "launcher.py")
	capturePath := filepath.Join(dir, "capture.json")
	launcherScript := `#!/usr/bin/env python3
import json
import os
import sys
from pathlib import Path

Path(os.environ["WRAPPER_CAPTURE"]).write_text(json.dumps({
    "args": sys.argv[1:],
    "desktop_startup_id": os.environ.get("DESKTOP_STARTUP_ID"),
    "xdg_data_dirs": os.environ.get("XDG_DATA_DIRS"),
}))
`
	if err := os.WriteFile(launcher, []byte(launcherScript), testExecutablePerm); err != nil {
		t.Fatal(err)
	}
	flagsFile := filepath.Join(dir, "browser-flags.conf")
	if err := os.WriteFile(
		flagsFile,
		[]byte("# runtime flags\n--from-file 'two words'\n"),
		testPrivatePerm,
	); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(dir, "browser")
	if err := WriteWrapper(
		wrapper,
		launcher,
		filepath.Base(flagsFile),
		[]string{"--configured", "configured two"},
		[]string{"--extra"},
	); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(wrapper, "--runtime", "runtime two")
	command.Env = slices.DeleteFunc(os.Environ(), func(value string) bool {
		return strings.HasPrefix(value, "WRAPPER_CAPTURE=") ||
			strings.HasPrefix(value, "DESKTOP_STARTUP_ID=") ||
			strings.HasPrefix(value, "XDG_DATA_DIRS=") ||
			strings.HasPrefix(value, "XDG_CONFIG_HOME=")
	})
	command.Env = append(
		command.Env,
		"WRAPPER_CAPTURE="+capturePath,
		"DESKTOP_STARTUP_ID=stale-token",
		"XDG_DATA_DIRS=/custom/share",
		"XDG_CONFIG_HOME="+dir,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run generated wrapper: %v\n%s", err, output)
	}

	data, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	var captured struct {
		Args             []string `json:"args"`
		DesktopStartupID *string  `json:"desktop_startup_id"`
		XDGDataDirs      string   `json:"xdg_data_dirs"`
	}
	if err := json.Unmarshal(data, &captured); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{
		"--configured", "configured two", "--extra",
		"--from-file", "two words", "--runtime", "runtime two",
	}
	if !slices.Equal(captured.Args, wantArgs) {
		t.Fatalf("wrapper args = %#v, want %#v", captured.Args, wantArgs)
	}
	if captured.DesktopStartupID != nil {
		t.Fatalf("DESKTOP_STARTUP_ID was not cleared: %q", *captured.DesktopStartupID)
	}
	for _, want := range []string{"/custom/share", "/usr/local/share", "/usr/share"} {
		if !slices.Contains(filepath.SplitList(captured.XDGDataDirs), want) {
			t.Fatalf("XDG_DATA_DIRS = %q, missing %q", captured.XDGDataDirs, want)
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

	got, err := LinuxDesktopEntry(
		input,
		"/home/user/.local/bin/helium-browser",
		"helium",
		"helium-browser",
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"Exec           = /home/user/.local/bin/helium-browser %U",
		"StartupNotify  = false",
		"StartupWMClass = helium-browser\n",
		"Exec = /home/user/.local/bin/helium-browser\n",
		"Exec = /home/user/.local/bin/helium-browser --incognito",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("desktop entry %q does not contain %q", got, want)
		}
	}
}
