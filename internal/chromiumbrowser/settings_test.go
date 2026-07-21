package chromiumbrowser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyProfileSettingsContinuesWhenExtensionStorageIsBroken(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "Default")
	storageDir := filepath.Join(profileDir, "Local Extension Settings", "broken-extension")
	if err := os.MkdirAll(storageDir, testDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(storageDir, "CURRENT"),
		[]byte("MANIFEST-000001\n"),
		testPrivatePerm,
	); err != nil {
		t.Fatal(err)
	}

	browser := Config{
		ExecutableName: testExecutableName,
		Paths: map[string]ModePaths{
			"test": {ProfileDir: profileDir},
		},
	}.Browser()
	browser.PreferencePatches = append(
		browser.PreferencePatches,
		func(preferences map[string]any) {
			preferences["preferences-applied"] = true
		},
	)

	err := browser.ApplyProfileSettings(ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{
			{
				Name: "test settings",
				Data: []byte(`{
					"local": [
						{
							"id": "broken-extension",
							"values": {
								"enabled": true
							}
						}
					]
				}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	preferences, err := ReadPreferences(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	if preferences["preferences-applied"] != true {
		t.Fatalf("preferences-applied = %v, want true", preferences["preferences-applied"])
	}
	defaultContentSettings := NestedObject(preferences, "profile.default_content_setting_values")
	if got := defaultContentSettings["cookies"]; contentSettingInt(got) != testContentSettingSessionOnly {
		t.Fatalf("default cookie setting = %v, want 4", got)
	}

	localState, err := ReadLocalState(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := localState["hardware_acceleration_mode_previous"]; got != true {
		t.Fatalf("hardware acceleration setting = %v, want true", got)
	}
}

func TestApplyProfileSettingsUsesCallerSuppliedCookieAllowlist(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	browser := Config{ExecutableName: testExecutableName}.Browser()
	if err := browser.ApplyProfileSettings(ApplyOptions{
		ProfileDir: profileDir,
		Input: ApplyInput{
			CookieAllowlist: []string{"[*.]example.com"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	preferences, err := ReadPreferences(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	exceptions := NestedObject(preferences, chromiumCookieExceptionsPath)
	entry, ok := exceptions["[*.]example.com,*"].(map[string]any)
	if !ok || contentSettingInt(entry[chromiumContentSettingKey]) != chromiumContentSettingAllow {
		t.Fatalf("caller-supplied cookie exception = %#v", exceptions["[*.]example.com,*"])
	}
}

func TestApplyExtensionSettingsRejectsTrailingJSON(t *testing.T) {
	err := ApplyExtensionSettings(ApplyOptions{
		ProfileDir: t.TempDir(),
		SettingsSource: []SettingsSource{{
			Name: "invalid settings",
			Data: []byte(`{"local":[],"sync":[]} {}`),
		}},
	})
	if err == nil {
		t.Fatal("expected trailing JSON to fail")
	}
}

func TestReadPreferencesRejectsTrailingJSON(t *testing.T) {
	profileDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(profileDir, PreferencesFilename),
		[]byte(`{} {}`),
		testPrivatePerm,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPreferences(profileDir); err == nil {
		t.Fatal("expected trailing JSON to fail")
	}
}
