package helium

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigBuildsHeliumPathsAndDownloads(t *testing.T) {
	t.Setenv("HOME", "/home/helium-test")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")

	browser := defaultConfig.Browser.Browser()
	if browser.ExecutableName != "helium-browser" {
		t.Fatalf("executable name = %q, want helium-browser", browser.ExecutableName)
	}
	if browser.AliasName != "helium" {
		t.Fatalf("alias name = %q, want helium", browser.AliasName)
	}
	if browser.MacOSAppDir != filepath.FromSlash("/Applications/Helium.app") {
		t.Fatalf("macOS app dir = %q", browser.MacOSAppDir)
	}
	if got := defaultConfig.Browser.DefaultProfileDir("linux"); got != filepath.FromSlash("/xdg/config/net.imput.helium/Default") {
		t.Fatalf("linux profile dir = %q", got)
	}
	externalDirs := defaultConfig.Browser.ExternalExtensionDirs("macos")
	if len(externalDirs) != 2 {
		t.Fatalf("macOS external extension dir count = %d, want 2", len(externalDirs))
	}
	if externalDirs[0] != filepath.FromSlash("/home/helium-test/Library/Application Support/net.imput.helium/External Extensions") {
		t.Fatalf("first macOS external extension dir = %q", externalDirs[0])
	}

	preferences := map[string]any{}
	for _, patch := range browser.PreferencePatches {
		patch(preferences)
	}
	heliumPrefs := preferences["helium"].(map[string]any)
	if got := heliumPrefs["completed_onboarding"]; got != true {
		t.Fatalf("completed onboarding = %v, want true", got)
	}
	services := heliumPrefs["services"].(map[string]any)
	for _, key := range []string{
		"user_consented",
		"enabled",
		"ext_proxy",
		"bangs",
		"spellcheck_files",
		"browser_updates",
		"ublock_assets",
	} {
		if got := services[key]; got != true {
			t.Fatalf("helium.services.%s = %v, want true", key, got)
		}
	}
	if got := services["schema_version"]; got != int64(1) {
		t.Fatalf("helium.services.schema_version = %v, want 1", got)
	}
	profilePrefs := preferences["profile"].(map[string]any)
	defaultContentSettings := profilePrefs["default_content_setting_values"].(map[string]any)
	if got := defaultContentSettings["cookies"]; got != int64(4) {
		t.Fatalf("profile.default_content_setting_values.cookies = %v, want 4", got)
	}
	localState := map[string]any{}
	for _, patch := range browser.LocalStatePatches {
		patch(localState)
	}
	localHeliumPrefs := localState["helium"].(map[string]any)
	localBrowserPrefs := localHeliumPrefs["browser"].(map[string]any)
	if got := localBrowserPrefs["default_browser_infobar_rejected"]; got != true {
		t.Fatalf("helium.browser.default_browser_infobar_rejected = %v, want true", got)
	}
	if got := localState["hardware_acceleration_mode_previous"]; got != true {
		t.Fatalf("hardware_acceleration_mode_previous = %v, want true", got)
	}
	if got := localState["variations_crash_streak"]; got != int64(0) {
		t.Fatalf("variations_crash_streak = %v, want 0", got)
	}
	stability := localState["user_experience_metrics"].(map[string]any)["stability"].(map[string]any)
	if got := stability["exited_cleanly"]; got != true {
		t.Fatalf("user_experience_metrics.stability.exited_cleanly = %v, want true", got)
	}
	variations := map[string]any{}
	for _, patch := range browser.VariationPatches {
		patch(variations)
	}
	if got := variations["user_experience_metrics.stability.exited_cleanly"]; got != true {
		t.Fatalf("variation exited_cleanly = %v, want true", got)
	}
	if got := variations["variations_crash_streak"]; got != int64(0) {
		t.Fatalf("variation variations_crash_streak = %v, want 0", got)
	}
}

func TestLoadConfigRejectsInvalidExtensionID(t *testing.T) {
	_, err := loadConfig([]byte(`
[browser]
executable_name = "helium-browser"

[browser.macos]
app_dir = "/Applications/Helium.app"
launcher_path = "Contents/MacOS/Helium"

[browser.paths.macos]
profile_dir = "/tmp/Default"

[browser.paths.linux]
profile_dir = "/tmp/Default"

[browser.extensions]
refined_github_id = "bad"

`), "test Helium defaults")
	if err == nil {
		t.Fatal("expected invalid extension ID error")
	}
}
