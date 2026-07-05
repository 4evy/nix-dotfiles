package chromiumbrowser

import (
	"fmt"

	"github.com/4evy/dotfiles/internal/common/chromiumext"
)

type InstallOptions struct {
	Mode           string
	Root           string
	AppDir         string
	BinDir         string
	Flags          string
	Settings       []string
	SettingsSource []SettingsSource
	BundlePatches  []chromiumext.BundlePatch
	ApplySettings  bool

	extraWrapperFlags  []string
	extensionIDAliases map[string]string
}

func (browser Browser) Install(options InstallOptions) error {
	normalized, err := browser.normalized()
	if err != nil {
		return err
	}
	switch options.Mode {
	case "macos":
		return normalized.installMacOS(&options)
	case "linux":
		return normalized.installLinux(&options)
	default:
		return fmt.Errorf("unsupported installer mode: %s", options.Mode)
	}
}

func (browser Browser) applyInstallSettings(options *InstallOptions) error {
	if !options.ApplySettings {
		return nil
	}
	profile := browser.DefaultProfileDir(options.Mode)
	if profile == "" {
		return nil
	}
	return browser.ApplyProfileSettings(ApplyOptions{
		ProfileDir:         profile,
		Settings:           options.Settings,
		SettingsSource:     options.SettingsSource,
		ExtensionIDAliases: options.extensionIDAliases,
		GitHubToken:        true,
	})
}
