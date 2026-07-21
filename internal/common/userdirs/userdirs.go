package userdirs

import (
	"cmp"
	"path/filepath"

	"github.com/4evy/dotfiles/internal/common/envx"
	"github.com/adrg/xdg"
)

const (
	defaultConfigHome = ".config"
	defaultDataHome   = ".local/share"
	defaultStateHome  = ".local/state"
	defaultCacheHome  = ".cache"
	defaultBinHome    = ".local/bin"
)

type environment struct {
	ConfigHome string `env:"XDG_CONFIG_HOME"`
	DataHome   string `env:"XDG_DATA_HOME"`
	StateHome  string `env:"XDG_STATE_HOME"`
	CacheHome  string `env:"XDG_CACHE_HOME"`
	BinHome    string `env:"XDG_BIN_HOME"`
}

func ConfigHome(home string) string {
	environment := envx.MustParse[environment]()
	return cmp.Or(environment.ConfigHome,
		homeRelative(home, xdg.ConfigHome, defaultConfigHome))
}

func DataHome(home string) string {
	environment := envx.MustParse[environment]()
	return cmp.Or(environment.DataHome,
		homeRelative(home, xdg.DataHome, defaultDataHome))
}

func StateHome(home string) string {
	environment := envx.MustParse[environment]()
	return cmp.Or(environment.StateHome,
		homeRelative(home, xdg.StateHome, defaultStateHome))
}

func CacheHome(home string) string {
	environment := envx.MustParse[environment]()
	return cmp.Or(environment.CacheHome,
		homeRelative(home, xdg.CacheHome, defaultCacheHome))
}

func BinHome(home string) string {
	environment := envx.MustParse[environment]()
	return cmp.Or(environment.BinHome,
		homeRelative(home, xdg.BinHome, defaultBinHome))
}

func homeRelative(home, detected, rel string) string {
	if home != "" {
		return filepath.Join(home, rel)
	}
	if detected != "" {
		return detected
	}
	return filepath.Join(".", rel)
}
