package chromiumbrowser

import (
	_ "embed"
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

const embeddedChromiumDefaultsName = "embedded Chromium defaults"

//go:embed defaults.toml
var defaultConfigData []byte

var defaultConfig = mustLoadDefaultConfig(defaultConfigData)

func mustLoadDefaultConfig(data []byte) Config {
	config, err := loadDefaultConfig(data, embeddedChromiumDefaultsName)
	if err != nil {
		panic(err)
	}
	return config
}

func loadDefaultConfig(data []byte, name string) (Config, error) {
	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", name, err)
	}
	return config, nil
}
