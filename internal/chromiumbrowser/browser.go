package chromiumbrowser

import (
	"fmt"
	"path/filepath"

	"github.com/4evy/dotfiles/internal/common/envx"
)

type environment struct {
	Home string `env:"HOME"`
}

type Browser struct {
	Name              string
	LogPrefix         string
	ExecutableName    string
	AliasName         string
	LinuxDesktopID    string
	LinuxWrapperFlags []string
	LinuxLauncherName string
	LinuxDesktopName  string
	LinuxDesktopExec  string
	LinuxIconName     string
	LinuxIconSource   string
	MacOSAppDir       string
	MacOSLauncherPath string
	ExternalDirs      func(mode string) []string
	DefaultProfileDir func(mode string) string
	PreferencePatches []PreferencePatch
	LocalStatePatches []PreferencePatch
	VariationPatches  []PreferencePatch
	ExtensionIDs      ExtensionIDs
}

func (browser Browser) normalized() (Browser, error) {
	if browser.Name == "" {
		browser.Name = "Chromium"
	}
	if browser.LogPrefix == "" {
		browser.LogPrefix = "chromium"
	}
	if browser.ExecutableName == "" {
		return Browser{}, fmt.Errorf("%s browser config is missing ExecutableName", browser.Name)
	}
	if browser.LinuxLauncherName == "" {
		browser.LinuxLauncherName = browser.ExecutableName
	}
	if browser.LinuxDesktopExec == "" {
		browser.LinuxDesktopExec = browser.ExecutableName
	}
	if browser.LinuxDesktopName == "" {
		browser.LinuxDesktopName = browser.ExecutableName + ".desktop"
	}
	if browser.LinuxIconName == "" {
		browser.LinuxIconName = browser.ExecutableName + ".png"
	}
	if browser.LinuxIconSource == "" {
		browser.LinuxIconSource = "product_logo_256.png"
	}
	if browser.MacOSLauncherPath == "" {
		browser.MacOSLauncherPath = filepath.Join("Contents", "MacOS", browser.Name)
	}
	if browser.ExternalDirs == nil {
		browser.ExternalDirs = func(string) []string { return nil }
	}
	if browser.DefaultProfileDir == nil {
		browser.DefaultProfileDir = func(string) string { return "" }
	}
	return browser, nil
}

func (browser Browser) defaultAppDir(root string) string {
	return filepath.Join(root, "app")
}

func homeDir() string {
	return envx.MustParse[environment]().Home
}
