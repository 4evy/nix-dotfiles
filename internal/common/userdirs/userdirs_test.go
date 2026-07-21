package userdirs

import (
	"path/filepath"
	"testing"
)

const (
	configDirectory = "config"
	dataDirectory   = "data"
	stateDirectory  = "state"
	cacheDirectory  = "cache"
	binDirectory    = "bin"
	configHomeEnv   = "XDG_CONFIG_HOME"
	dataHomeEnv     = "XDG_DATA_HOME"
	stateHomeEnv    = "XDG_STATE_HOME"
	cacheHomeEnv    = "XDG_CACHE_HOME"
	binHomeEnv      = "XDG_BIN_HOME"
)

func TestDirectoriesPreferExplicitXDGEnvironment(t *testing.T) {
	t.Setenv(configHomeEnv, "/xdg/config")
	t.Setenv(dataHomeEnv, "/xdg/data")
	t.Setenv(stateHomeEnv, "/xdg/state")
	t.Setenv(cacheHomeEnv, "/xdg/cache")
	t.Setenv(binHomeEnv, "/xdg/bin")

	tests := map[string]string{
		configDirectory: ConfigHome("/ignored"),
		dataDirectory:   DataHome("/ignored"),
		stateDirectory:  StateHome("/ignored"),
		cacheDirectory:  CacheHome("/ignored"),
		binDirectory:    BinHome("/ignored"),
	}
	wants := map[string]string{
		configDirectory: "/xdg/config",
		dataDirectory:   "/xdg/data",
		stateDirectory:  "/xdg/state",
		cacheDirectory:  "/xdg/cache",
		binDirectory:    "/xdg/bin",
	}
	for name, got := range tests {
		if got != wants[name] {
			t.Errorf("%s home = %q, want %q", name, got, wants[name])
		}
	}
}

func TestDirectoriesFallBackUnderRequestedHome(t *testing.T) {
	for _, name := range []string{
		configHomeEnv,
		dataHomeEnv,
		stateHomeEnv,
		cacheHomeEnv,
		binHomeEnv,
	} {
		t.Setenv(name, "")
	}

	home := filepath.Join(string(filepath.Separator), "users", "test")
	tests := map[string]string{
		configDirectory: ConfigHome(home),
		dataDirectory:   DataHome(home),
		stateDirectory:  StateHome(home),
		cacheDirectory:  CacheHome(home),
		binDirectory:    BinHome(home),
	}
	wants := map[string]string{
		configDirectory: filepath.Join(home, defaultConfigHome),
		dataDirectory:   filepath.Join(home, defaultDataHome),
		stateDirectory:  filepath.Join(home, defaultStateHome),
		cacheDirectory:  filepath.Join(home, defaultCacheHome),
		binDirectory:    filepath.Join(home, defaultBinHome),
	}
	for name, got := range tests {
		if got != wants[name] {
			t.Errorf("%s home = %q, want %q", name, got, wants[name])
		}
	}
}
