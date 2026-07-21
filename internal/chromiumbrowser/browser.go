package chromiumbrowser

import (
	"cmp"
	"fmt"
	"path/filepath"

	"github.com/4evy/dotfiles/internal/common/chromiumext"
	"github.com/4evy/dotfiles/internal/common/envx"
)

const (
	defaultBrowserName      = "Chromium"
	defaultBrowserLogPrefix = "chromium"
	defaultLinuxIconSource  = "product_logo_256.png"
	defaultMacOSContentsDir = "Contents"
	defaultMacOSLauncherDir = "MacOS"
	desktopEntryFileSuffix  = ".desktop"
	pngFileExtension        = ".png"
)

type environment struct {
	Home string `env:"HOME"`
}

type Browser struct {
	Config            Config
	DefaultSettings   []SettingsSource
	PreferencePatches []PreferencePatch
	LocalStatePatches []PreferencePatch
	VariationPatches  []PreferencePatch
}

func (browser Browser) normalized() (Browser, error) {
	config := &browser.Config
	for sourceID, installedID := range config.ExtensionIDAliases {
		if !chromiumext.ValidExtensionID(sourceID) {
			return Browser{}, fmt.Errorf("invalid extension ID alias source %q", sourceID)
		}
		if !chromiumext.ValidExtensionID(installedID) {
			return Browser{}, fmt.Errorf(
				"invalid installed extension ID %q for alias %q",
				installedID,
				sourceID,
			)
		}
	}
	config.Name = cmp.Or(config.Name, defaultBrowserName)
	config.LogPrefix = cmp.Or(config.LogPrefix, defaultBrowserLogPrefix)
	if config.ExecutableName == "" {
		return Browser{}, fmt.Errorf("%s browser config is missing executable_name", config.Name)
	}
	config.Linux.LauncherName = cmp.Or(config.Linux.LauncherName, config.ExecutableName)
	config.Linux.DesktopExec = cmp.Or(config.Linux.DesktopExec, config.ExecutableName)
	config.Linux.DesktopName = cmp.Or(
		config.Linux.DesktopName,
		config.ExecutableName+desktopEntryFileSuffix,
	)
	config.Linux.IconName = cmp.Or(
		config.Linux.IconName,
		config.ExecutableName+pngFileExtension,
	)
	config.Linux.IconSource = cmp.Or(config.Linux.IconSource, defaultLinuxIconSource)
	config.MacOS.LauncherPath = cmp.Or(
		config.MacOS.LauncherPath,
		filepath.Join(defaultMacOSContentsDir, defaultMacOSLauncherDir, config.Name),
	)
	return browser, nil
}

func homeDir() string {
	return envx.MustParse[environment]().Home
}
