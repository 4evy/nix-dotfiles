package helium

import (
	"testing"

	"github.com/4evy/dotfiles/internal/chromiumbrowser"
)

func TestLoadConfigRejectsMissingLinuxProfile(t *testing.T) {
	_, err := loadConfig([]byte(`
[browser]
executable_name = "helium-browser"

[browser.macos]
app_dir = "/Applications/Helium.app"
launcher_path = "Contents/MacOS/Helium"

[browser.paths.macos]
profile_dir = "/tmp/Default"
`), "test Helium defaults")
	if err == nil {
		t.Fatal("expected missing Linux profile error")
	}
}

func TestDefaultConfigMatchesCurrentHeliumServiceSchema(t *testing.T) {
	browser := DefaultBrowser()
	preferences := map[string]any{}
	for _, patch := range browser.PreferencePatches {
		patch(preferences)
	}
	services := chromiumbrowser.NestedObject(preferences, "helium.services")
	for _, key := range []string{
		"enabled",
		"user_consented",
		"ext_proxy",
		"bangs",
		"spellcheck_files",
		"browser_updates",
		"ublock_assets",
	} {
		if got := services[key]; got != true {
			t.Errorf("helium.services.%s = %v, want true", key, got)
		}
	}
	if got := services["schema_version"]; got != int64(1) {
		t.Errorf("helium.services.schema_version = %v, want 1", got)
	}
	if got := chromiumbrowser.NestedObject(preferences, "helium")["completed_onboarding"]; got != true {
		t.Errorf("helium.completed_onboarding = %v, want true", got)
	}
}

func TestDefaultConfigDisablesHeliumCrashReporting(t *testing.T) {
	browser := DefaultBrowser()
	localState := map[string]any{}
	for _, patch := range browser.LocalStatePatches {
		patch(localState)
	}
	crashReporting := chromiumbrowser.NestedObject(localState, "helium.crash_reporting")
	if got := crashReporting["mode"]; got != int64(-1) {
		t.Fatalf("helium.crash_reporting.mode = %v, want -1 (disabled)", got)
	}
}

func TestDefaultFlagsFileIsRelativeToChromiumConfigHome(t *testing.T) {
	if got := defaultConfig.Browser.FlagsFile; got != "helium-flags.conf" {
		t.Fatalf("browser.flags_file = %q, want helium-flags.conf", got)
	}
}

func TestDefaultConfigMapsCatalogExtensionToBundledComponent(t *testing.T) {
	aliases := DefaultBrowser().Config.ExtensionIDAliases
	if got := aliases["cjpalhdlnbpafiamejdnhcphjbkeiagm"]; got != "blockjmkbacgjkknlgpkjjiijinjdanf" {
		t.Fatalf("bundled component ID = %q", got)
	}
}
