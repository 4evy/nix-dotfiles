package helium

import (
	_ "embed"
	"fmt"

	"github.com/4evy/dotfiles/internal/chromiumbrowser"
	"github.com/pelletier/go-toml/v2"
)

const embeddedHeliumDefaultsName = "embedded Helium defaults"

//go:embed defaults.toml
var defaultConfigData []byte

var defaultConfig = mustLoadConfig(defaultConfigData)

type Config struct {
	Browser chromiumbrowser.Config `toml:"browser"`
}

func mustLoadConfig(data []byte) Config {
	config, err := loadConfig(data, embeddedHeliumDefaultsName)
	if err != nil {
		panic(err)
	}
	return config
}

func loadConfig(data []byte, name string) (Config, error) {
	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", name, err)
	}
	if err := config.validate(name); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) validate(name string) error {
	if config.Browser.ExecutableName == "" {
		return fmt.Errorf("%s is missing browser.executable_name", name)
	}
	if config.Browser.MacOS.AppDir == "" {
		return fmt.Errorf("%s is missing browser.macos.app_dir", name)
	}
	if config.Browser.MacOS.LauncherPath == "" {
		return fmt.Errorf("%s is missing browser.macos.launcher_path", name)
	}
	for _, mode := range []string{chromiumbrowser.ModeMacOS, chromiumbrowser.ModeLinux} {
		paths, ok := config.Browser.Paths[mode]
		if !ok {
			return fmt.Errorf("%s is missing browser.paths.%s", name, mode)
		}
		if paths.ProfileDir == "" {
			return fmt.Errorf("%s is missing browser.paths.%s.profile_dir", name, mode)
		}
	}
	return nil
}

func DefaultBrowser() chromiumbrowser.Browser {
	browser := defaultConfig.Browser.Browser()
	browser.DefaultSettings = append(browser.DefaultSettings, heliumDefaultSettings...)
	return browser
}
