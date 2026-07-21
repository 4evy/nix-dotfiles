package chromiumbrowser

import (
	"path/filepath"
	"testing"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	"github.com/pelletier/go-toml/v2"
	"gotest.tools/v3/assert"
)

const (
	testExecutableName = "test-browser"
	testDirPerm        = fileutil.DefaultDirPerm
	testExecutablePerm = fileutil.DefaultFilePerm | fileutil.ExecutablePerm
	testPrivatePerm    = fileutil.PrivateFilePerm
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
flags_file = "example-flags.conf"

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

[extension_id_aliases]
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

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
	assert.Equal(t, browser.Config.Name, "Example")
	assert.Equal(t, browser.Config.LogPrefix, "example-browser")
	assert.Equal(t, browser.Config.ExecutableName, "example-browser")
	assert.Equal(t, browser.Config.AliasName, "example")
	assert.Equal(t, browser.Config.Linux.DesktopID, "example-browser")
	assert.DeepEqual(t, browser.Config.Linux.WrapperFlags, []string{"--no-first-run"})
	assert.Equal(t, browser.Config.Linux.LauncherName, "example-launcher")
	assert.Equal(t, browser.Config.Linux.DesktopName, "example.desktop")
	assert.Equal(t, browser.Config.Linux.DesktopExec, "example")
	assert.Equal(t, browser.Config.Linux.IconName, "example.png")
	assert.Equal(t, browser.Config.Linux.IconSource, "icons/example.png")
	assert.Equal(t, expandPathTemplate(browser.Config.MacOS.AppDir), filepath.FromSlash("/home/browser-test/Applications/Example.app"))
	assert.Equal(t, filepath.FromSlash(browser.Config.MacOS.LauncherPath), filepath.Join("Contents", "MacOS", "Example"))
	assert.Equal(t, browser.Config.FlagsFile, "example-flags.conf")
	assert.Equal(t, browser.Config.DefaultProfileDir("linux"), filepath.FromSlash("/xdg/config/example/Default"))
	assert.Equal(
		t,
		browser.Config.ExtensionIDAliases["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	)
	assert.DeepEqual(
		t,
		browser.Config.ExternalExtensionDirs("linux"),
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
	assert.DeepEqual(
		t,
		cookieExceptions["[*.]example.com,*"],
		map[string]any{chromiumContentSettingKey: chromiumContentSettingAllow},
	)

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

func TestBrowserRejectsInvalidExtensionIDAlias(t *testing.T) {
	browser := Config{
		ExecutableName: testExecutableName,
		ExtensionIDAliases: map[string]string{
			"bad": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}.Browser()
	_, err := browser.normalized()
	if err == nil {
		t.Fatal("expected invalid extension ID error")
	}
}
