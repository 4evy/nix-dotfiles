package helium

import (
	"fmt"

	"github.com/4evy/dotfiles/internal/chromiumbrowser"
	"github.com/buildkite/shellwords"
)

type ConfigureOptions struct {
	Mode          string
	Root          string
	AppDir        string
	BinDir        string
	Flags         string
	Settings      []string
	Input         chromiumbrowser.ApplyInput
	ApplySettings bool
}

func ConfigureInstalled(options ConfigureOptions) error {
	if options.AppDir == "" {
		return fmt.Errorf("helium application directory is required")
	}
	return configureBrowser(options)
}

func configureBrowser(options ConfigureOptions) error {
	flags, err := shellwords.SplitPosix(options.Flags)
	if err != nil {
		return fmt.Errorf("parse Helium browser flags: %w", err)
	}
	browser := DefaultBrowser()
	browser.PreferencePatches = append(
		browser.PreferencePatches,
		func(preferences map[string]any) {
			applyThemePreferencesFromFlags(preferences, flags)
		},
	)
	return browser.Install(chromiumbrowser.InstallOptions{
		Mode:          options.Mode,
		Root:          options.Root,
		AppDir:        options.AppDir,
		BinDir:        options.BinDir,
		Flags:         flags,
		Settings:      options.Settings,
		Input:         options.Input,
		ApplySettings: options.ApplySettings,
	})
}
