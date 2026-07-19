package chromiumbrowser

import (
	"charm.land/log/v2"
)

type ApplyOptions struct {
	ProfileDir         string
	Settings           []string
	SettingsSource     []SettingsSource
	ExtensionIDAliases map[string]string
	GitHubToken        bool
	TokenFunc          func() string
	ExtensionIDs       ExtensionIDs
}

type SettingsSource struct {
	Name string
	Data []byte
}

type ExtensionIDs struct {
	RefinedGitHub string `toml:"refined_github_id"`
}

func (ids ExtensionIDs) WithFallback(fallback ExtensionIDs) ExtensionIDs {
	if ids.RefinedGitHub == "" {
		ids.RefinedGitHub = fallback.RefinedGitHub
	}
	return ids
}

func (browser Browser) ApplyProfileSettings(options ApplyOptions) error {
	normalized, err := browser.normalized()
	if err != nil {
		return err
	}
	options.ExtensionIDs = options.ExtensionIDs.WithFallback(normalized.ExtensionIDs)

	err = normalized.ApplyExtensionSettings(options)
	if err != nil && !isStorageTemporarilyUnavailable(err) {
		return err
	}
	if err != nil {
		log.Warn(
			normalized.LogPrefix+": extension settings storage is locked; continuing without applying extension settings",
			"error",
			err,
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
	options.ExtensionIDs = options.ExtensionIDs.WithFallback(normalized.ExtensionIDs)
	return ApplyExtensionSettings(options)
}
