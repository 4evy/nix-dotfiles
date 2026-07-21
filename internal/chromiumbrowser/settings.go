package chromiumbrowser

import (
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"charm.land/log/v2"
)

type ApplyOptions struct {
	ProfileDir         string
	Settings           []string
	SettingsSource     []SettingsSource
	ExtensionIDAliases map[string]string
	Input              ApplyInput
}

type ApplyInput struct {
	CookieAllowlist []string          `json:"cookie_allowlist"`
	ExtensionValues map[string]string `json:"extension_values"`
}

type SettingsSource struct {
	Name string
	Data []byte
}

const (
	defaultSettingsDir = "settings/default"
	jsonFileGlob       = "*.json"
	embeddedNamePrefix = "embedded "
)

//go:embed settings/default/*.json
var defaultSettingsFS embed.FS

func DefaultSettingsSources() ([]SettingsSource, error) {
	return SettingsSourcesFromFS(defaultSettingsFS, defaultSettingsDir+"/"+jsonFileGlob)
}

func SettingsSourcesFromFS(settingsFS fs.FS, pattern string) ([]SettingsSource, error) {
	paths, err := fs.Glob(settingsFS, pattern)
	if err != nil {
		return nil, fmt.Errorf("find extension settings matching %s: %w", pattern, err)
	}
	sources := make([]SettingsSource, 0, len(paths))
	for _, path := range paths {
		data, err := fs.ReadFile(settingsFS, path)
		if err != nil {
			return nil, fmt.Errorf("read extension settings file %s: %w", path, err)
		}
		sources = append(sources, SettingsSource{Name: embeddedNamePrefix + path, Data: data})
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("extension settings pattern %s matched no files", pattern)
	}
	return sources, nil
}

func (browser Browser) ApplyProfileSettings(options ApplyOptions) error {
	normalized, err := browser.normalized()
	if err != nil {
		return err
	}
	err = normalized.ApplyExtensionSettings(options)
	if err != nil && !isStorageTemporarilyUnavailable(err) {
		return err
	}
	if err != nil {
		log.Warn(
			normalized.Config.LogPrefix+": extension settings storage is unavailable; continuing without applying extension settings",
			"error",
			err,
		)
	}
	if options.Input.CookieAllowlist != nil {
		normalized.PreferencePatches = append(
			normalized.PreferencePatches,
			func(preferences map[string]any) {
				SetCookieAllowlist(preferences, options.Input.CookieAllowlist)
			},
		)
	}
	if err := normalized.ApplyBrowserPreferenceSettings(options.ProfileDir); err != nil {
		return err
	}
	if err := normalized.ApplyBrowserLocalStateSettings(options.ProfileDir); err != nil {
		return err
	}
	return normalized.ApplyBrowserVariationSettings(options.ProfileDir)
}

func (browser Browser) ApplyExtensionSettings(options ApplyOptions) error {
	normalized, err := browser.normalized()
	if err != nil {
		return err
	}
	defaults, err := DefaultSettingsSources()
	if err != nil {
		return err
	}
	options.SettingsSource = slices.Concat(
		defaults,
		normalized.DefaultSettings,
		options.SettingsSource,
	)
	options.ExtensionIDAliases = mergeExtensionIDAliases(
		normalized.Config.ExtensionIDAliases,
		options.ExtensionIDAliases,
	)
	return ApplyExtensionSettings(options)
}

func mergeExtensionIDAliases(
	defaults map[string]string,
	overrides map[string]string,
) map[string]string {
	aliases := make(map[string]string, len(defaults)+len(overrides))
	maps.Copy(aliases, defaults)
	maps.Copy(aliases, overrides)
	return aliases
}
