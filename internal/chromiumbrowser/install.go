package chromiumbrowser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/4evy/dotfiles/internal/common/fileutil"
)

const (
	// ModeMacOS selects the macOS browser installation flow.
	ModeMacOS = "macos"
	// ModeLinux selects the Linux browser installation flow.
	ModeLinux = "linux"
)

type InstallOptions struct {
	Mode           string
	Root           string
	AppDir         string
	BinDir         string
	Flags          []string
	Settings       []string
	SettingsSource []SettingsSource
	Input          ApplyInput
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
	case ModeMacOS:
		return normalized.installMacOS(&options)
	case ModeLinux:
		return normalized.installLinux(&options)
	default:
		return fmt.Errorf("unsupported installer mode: %s", options.Mode)
	}
}

func (browser Browser) applyInstallSettings(options *InstallOptions) error {
	if !options.ApplySettings {
		return nil
	}
	profile := browser.Config.DefaultProfileDir(options.Mode)
	if profile == "" {
		return nil
	}
	return browser.ApplyProfileSettings(ApplyOptions{
		ProfileDir:         profile,
		Settings:           options.Settings,
		SettingsSource:     options.SettingsSource,
		ExtensionIDAliases: options.extensionIDAliases,
		Input:              options.Input,
	})
}

func (browser Browser) prepareInstall(options *InstallOptions, appDir string) error {
	for _, dir := range []string{options.Root, options.BinDir} {
		if err := os.MkdirAll(dir, fileutil.DefaultDirPerm); err != nil {
			return err
		}
	}
	stat, err := os.Stat(appDir)
	if err != nil {
		return fmt.Errorf("find %s app directory %s: %w", browser.Config.Name, appDir, err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s app path is not a directory: %s", browser.Config.Name, appDir)
	}
	return nil
}

func (browser Browser) configureApp(
	options *InstallOptions,
	launcher string,
) error {
	if err := browser.installExtensions(options); err != nil {
		return err
	}
	if err := browser.applyInstallSettings(options); err != nil {
		return err
	}
	if err := writeWrapper(
		filepath.Join(options.BinDir, browser.Config.ExecutableName),
		launcher,
		browser.Config.FlagsFile,
		options,
	); err != nil {
		return err
	}
	if browser.Config.AliasName == "" {
		return nil
	}
	return replaceSymlink(
		browser.Config.ExecutableName,
		filepath.Join(options.BinDir, browser.Config.AliasName),
	)
}
