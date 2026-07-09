package userdirs

import (
	"path/filepath"
	"testing"
)

func TestDirectoriesPreferExplicitXDGEnvironment(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	t.Setenv("XDG_STATE_HOME", "/xdg/state")
	t.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	t.Setenv("XDG_BIN_HOME", "/xdg/bin")

	tests := map[string]string{
		"config": ConfigHome("/ignored"),
		"data":   DataHome("/ignored"),
		"state":  StateHome("/ignored"),
		"cache":  CacheHome("/ignored"),
		"bin":    BinHome("/ignored"),
	}
	wants := map[string]string{
		"config": "/xdg/config",
		"data":   "/xdg/data",
		"state":  "/xdg/state",
		"cache":  "/xdg/cache",
		"bin":    "/xdg/bin",
	}
	for name, got := range tests {
		if got != wants[name] {
			t.Errorf("%s home = %q, want %q", name, got, wants[name])
		}
	}
}

func TestDirectoriesFallBackUnderRequestedHome(t *testing.T) {
	for _, name := range []string{
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
		"XDG_STATE_HOME",
		"XDG_CACHE_HOME",
		"XDG_BIN_HOME",
	} {
		t.Setenv(name, "")
	}

	home := filepath.Join(string(filepath.Separator), "users", "test")
	tests := map[string]string{
		"config": ConfigHome(home),
		"data":   DataHome(home),
		"state":  StateHome(home),
		"cache":  CacheHome(home),
		"bin":    BinHome(home),
	}
	wants := map[string]string{
		"config": filepath.Join(home, ".config"),
		"data":   filepath.Join(home, ".local/share"),
		"state":  filepath.Join(home, ".local/state"),
		"cache":  filepath.Join(home, ".cache"),
		"bin":    filepath.Join(home, ".local/bin"),
	}
	for name, got := range tests {
		if got != wants[name] {
			t.Errorf("%s home = %q, want %q", name, got, wants[name])
		}
	}
}
