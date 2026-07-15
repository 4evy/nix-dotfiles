package chromiumbrowser

import (
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"gotest.tools/v3/assert"
)

func TestConfigBuildsBrowserFromTOML(t *testing.T) {
	t.Setenv("HOME", "/home/browser-test")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")

	var config Config
	err := toml.Unmarshal([]byte(`
name = "Example"
log_prefix = "example-browser"
executable_name = "example-browser"
alias_name = "example"

[linux]
desktop_id = "example-browser"
wrapper_flags = ["--no-first-run"]
launcher_name = "example-launcher"
desktop_name = "example.desktop"
desktop_exec = "example"
icon_name = "example.png"
icon_source = "icons/example.png"

[macos]
app_dir = "${home}/Applications/Example.app"
launcher_path = "Contents/MacOS/Example"

[paths.linux]
profile_dir = "${config_home}/example/Default"
external_extension_dirs = [
  "${config_home}/example/External Extensions",
]

[extensions]
refined_github_id = "github-extension"

[[preferences.values]]
path = "example.browser.enabled"
value = true

[[preferences.local_state_values]]
path = "example.browser.local_enabled"
value = true

[[preferences.variation_values]]
path = "example.browser.variation_enabled"
value = true

	[[preferences.accelerators]]
	path = "example.browser.custom_accelerators"
	command_id = "1"
	accelerator = "Control+One"

	[preferences.cookies]
	allow = ["[*.]example.com"]
	`), &config)
	assert.NilError(t, err)

	browser := config.Browser()
	assert.Equal(t, browser.Name, "Example")
	assert.Equal(t, browser.LogPrefix, "example-browser")
	assert.Equal(t, browser.ExecutableName, "example-browser")
	assert.Equal(t, browser.AliasName, "example")
	assert.Equal(t, browser.LinuxDesktopID, "example-browser")
	assert.DeepEqual(t, browser.LinuxWrapperFlags, []string{"--no-first-run"})
	assert.Equal(t, browser.LinuxLauncherName, "example-launcher")
	assert.Equal(t, browser.LinuxDesktopName, "example.desktop")
	assert.Equal(t, browser.LinuxDesktopExec, "example")
	assert.Equal(t, browser.LinuxIconName, "example.png")
	assert.Equal(t, browser.LinuxIconSource, "icons/example.png")
	assert.Equal(t, browser.MacOSAppDir, filepath.FromSlash("/home/browser-test/Applications/Example.app"))
	assert.Equal(t, browser.MacOSLauncherPath, filepath.Join("Contents", "MacOS", "Example"))
	assert.Equal(t, browser.DefaultProfileDir("linux"), filepath.FromSlash("/xdg/config/example/Default"))
	assert.Equal(t, browser.ExtensionIDs.RefinedGitHub, "github-extension")
	assert.DeepEqual(
		t,
		browser.ExternalDirs("linux"),
		[]string{filepath.FromSlash("/xdg/config/example/External Extensions")},
	)

	preferences := map[string]any{}
	for _, patch := range browser.PreferencePatches {
		patch(preferences)
	}
	exampleBrowser := preferences["example"].(map[string]any)["browser"].(map[string]any)
	assert.Equal(t, exampleBrowser["enabled"], true)
	customAccelerators := exampleBrowser["custom_accelerators"].(map[string]any)
	command := customAccelerators["1"].(map[string]any)
	assert.DeepEqual(t, command["added"], []any{"Control+One"})
	cookieExceptions := preferences["profile"].(map[string]any)["content_settings"].(map[string]any)["exceptions"].(map[string]any)["cookies"].(map[string]any)
	assert.DeepEqual(t, cookieExceptions["[*.]example.com,*"], map[string]any{"setting": 1})

	localState := map[string]any{}
	for _, patch := range browser.LocalStatePatches {
		patch(localState)
	}
	exampleLocalBrowser := localState["example"].(map[string]any)["browser"].(map[string]any)
	assert.Equal(t, exampleLocalBrowser["local_enabled"], true)

	variations := map[string]any{}
	for _, patch := range browser.VariationPatches {
		patch(variations)
	}
	assert.Equal(t, variations["example.browser.variation_enabled"], true)
}
