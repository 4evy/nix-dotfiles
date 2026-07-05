package helium

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/4evy/dotfiles/internal/chromiumbrowser"
	chromiumextensions "github.com/4evy/dotfiles/internal/chromiumbrowser/extensions"
	"github.com/4evy/dotfiles/internal/common/chromiumext"
)

const privateCookieAllowlistKey = `["helium-cookie-allowed-for-urls"]`

type InstallOptions struct {
	Mode          string
	Root          string
	AppDir        string
	BinDir        string
	Flags         string
	Settings      []string
	SecretsPath   string
	ApplySettings bool
}

func ConfigureInstalled(options InstallOptions) error {
	if options.AppDir == "" {
		return fmt.Errorf("helium application directory is required")
	}
	return installBrowser(options, options.AppDir)
}

func installBrowser(options InstallOptions, appDir string) error {
	settings, err := installSettingsSources(options)
	if err != nil {
		return err
	}
	browser := defaultConfig.Browser.Browser()
	browser.PreferencePatches = append(
		browser.PreferencePatches,
		func(preferences map[string]any) {
			applyThemePreferencesFromFlags(preferences, options.Flags)
		},
	)
	if cookieAllowlist, ok := privateCookieAllowlist(options.SecretsPath); ok {
		browser.PreferencePatches = append(
			browser.PreferencePatches,
			func(preferences map[string]any) {
				chromiumbrowser.SetCookieAllowlist(preferences, cookieAllowlist)
			},
		)
	}
	return browser.Install(chromiumbrowser.InstallOptions{
		Mode:           options.Mode,
		Root:           options.Root,
		AppDir:         appDir,
		BinDir:         options.BinDir,
		Flags:          options.Flags,
		Settings:       options.Settings,
		SettingsSource: settings,
		BundlePatches: []chromiumext.BundlePatch{
			chromiumextensions.DisableOpenOptionsPageCallsPatch,
		},
		ApplySettings: options.ApplySettings,
	})
}

func installSettingsSources(options InstallOptions) ([]SettingsSource, error) {
	if !options.ApplySettings {
		return nil, nil
	}
	settings, err := DefaultSettingsSources()
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func privateCookieAllowlist(secretsPath string) ([]string, bool) {
	if secretsPath == "" {
		return nil, false
	}
	if _, err := os.Stat(secretsPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "helium-browser: failed to stat private Chromium cookie allowlist: %v\n", err)
		}
		return nil, false
	}

	data, err := exec.Command(
		"sops",
		"-d",
		"--output-type",
		"json",
		"--extract",
		privateCookieAllowlistKey,
		secretsPath,
	).Output()
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) {
			fmt.Fprintf(os.Stderr, "helium-browser: failed to decrypt private Chromium cookie allowlist; continuing with public settings: %v\n", err)
		}
		return nil, false
	}

	var patterns []string
	if err := json.Unmarshal(data, &patterns); err != nil {
		fmt.Fprintf(os.Stderr, "helium-browser: failed to parse private Chromium cookie allowlist; continuing with public settings: %v\n", err)
		return nil, false
	}
	return patterns, true
}
