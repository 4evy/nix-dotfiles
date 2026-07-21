package chromiumbrowser

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestDefaultConfigOwnsChromiumPreferences(t *testing.T) {
	browser := Config{ExecutableName: testExecutableName}.Browser()
	assert.Equal(t, len(browser.Config.ExtensionIDAliases), 0)

	preferences := map[string]any{}
	for _, patch := range browser.PreferencePatches {
		patch(preferences)
	}
	defaultContentSettings := NestedObject(preferences, "profile.default_content_setting_values")
	assert.Equal(t, defaultContentSettings["cookies"], int64(testContentSettingSessionOnly))

	localState := map[string]any{}
	for _, patch := range browser.LocalStatePatches {
		patch(localState)
	}
	assert.Equal(t, localState["hardware_acceleration_mode_previous"], true)
	assert.Equal(t, localState["variations_crash_streak"], int64(0))

	variations := map[string]any{}
	for _, patch := range browser.VariationPatches {
		patch(variations)
	}
	assert.Equal(t, variations["variations_crash_streak"], int64(0))
}

func TestBrowserSpecificPreferencesOverrideChromiumDefaults(t *testing.T) {
	config := Config{
		ExecutableName: testExecutableName,
		Preferences: PreferenceDefaultsConfig{Values: []PreferenceValueConfig{
			{
				Path:  "profile.default_content_setting_values.cookies",
				Value: int64(chromiumContentSettingAllow),
			},
		}},
	}
	preferences := map[string]any{}
	for _, patch := range config.Browser().PreferencePatches {
		patch(preferences)
	}
	defaultContentSettings := NestedObject(preferences, "profile.default_content_setting_values")
	assert.Equal(
		t,
		defaultContentSettings["cookies"],
		int64(chromiumContentSettingAllow),
	)
}
