package chromiumbrowser

import (
	"os"
	"path/filepath"

	"github.com/4evy/dotfiles/internal/common/userdirs"
)

const (
	homeTemplateVariable       = "home"
	configHomeTemplateVariable = "config_home"
	dataHomeTemplateVariable   = "data_home"
)

type Config struct {
	Name               string                   `toml:"name"`
	LogPrefix          string                   `toml:"log_prefix"`
	ExecutableName     string                   `toml:"executable_name"`
	AliasName          string                   `toml:"alias_name"`
	FlagsFile          string                   `toml:"flags_file"`
	Linux              LinuxConfig              `toml:"linux"`
	MacOS              MacOSConfig              `toml:"macos"`
	Paths              map[string]ModePaths     `toml:"paths"`
	Preferences        PreferenceDefaultsConfig `toml:"preferences"`
	ExtensionIDAliases map[string]string        `toml:"extension_id_aliases"`
}

type LinuxConfig struct {
	DesktopID    string   `toml:"desktop_id"`
	WrapperFlags []string `toml:"wrapper_flags"`
	LauncherName string   `toml:"launcher_name"`
	DesktopName  string   `toml:"desktop_name"`
	DesktopExec  string   `toml:"desktop_exec"`
	IconName     string   `toml:"icon_name"`
	IconSource   string   `toml:"icon_source"`
}

type MacOSConfig struct {
	AppDir       string `toml:"app_dir"`
	LauncherPath string `toml:"launcher_path"`
}

type ModePaths struct {
	ProfileDir            string   `toml:"profile_dir"`
	ExternalExtensionDirs []string `toml:"external_extension_dirs"`
}

type PreferenceDefaultsConfig struct {
	Values           []PreferenceValueConfig       `toml:"values"`
	LocalStateValues []PreferenceValueConfig       `toml:"local_state_values"`
	VariationValues  []PreferenceValueConfig       `toml:"variation_values"`
	Accelerators     []PreferenceAcceleratorConfig `toml:"accelerators"`
	Cookies          CookiePreferenceConfig        `toml:"cookies"`
}

type PreferenceValueConfig struct {
	Path  string `toml:"path"`
	Value any    `toml:"value"`
}

type CookiePreferenceConfig struct {
	Allow []string `toml:"allow"`
}

type PreferenceAcceleratorConfig struct {
	Path        string `toml:"path"`
	CommandID   string `toml:"command_id"`
	Accelerator string `toml:"accelerator"`
}

func (config Config) Browser() Browser {
	browser := Browser{Config: config}
	if defaultConfig.Preferences.HasDefaults() {
		browser.PreferencePatches = append(
			browser.PreferencePatches,
			defaultConfig.Preferences.Patch,
		)
	}
	if config.Preferences.HasDefaults() {
		browser.PreferencePatches = append(browser.PreferencePatches, config.Preferences.Patch)
	}
	if defaultConfig.Preferences.HasLocalStateDefaults() {
		browser.LocalStatePatches = append(
			browser.LocalStatePatches,
			defaultConfig.Preferences.PatchLocalState,
		)
	}
	if config.Preferences.HasLocalStateDefaults() {
		browser.LocalStatePatches = append(
			browser.LocalStatePatches,
			config.Preferences.PatchLocalState,
		)
	}
	if defaultConfig.Preferences.HasVariationDefaults() {
		browser.VariationPatches = append(
			browser.VariationPatches,
			defaultConfig.Preferences.PatchVariations,
		)
	}
	if config.Preferences.HasVariationDefaults() {
		browser.VariationPatches = append(
			browser.VariationPatches,
			config.Preferences.PatchVariations,
		)
	}
	return browser
}

func (config Config) DefaultProfileDir(mode string) string {
	return expandPathTemplate(config.Paths[mode].ProfileDir)
}

func (config Config) ExternalExtensionDirs(mode string) []string {
	paths := config.Paths[mode].ExternalExtensionDirs
	if len(paths) == 0 {
		return nil
	}
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved = append(resolved, expandPathTemplate(path))
	}
	return resolved
}

func (config PreferenceDefaultsConfig) HasDefaults() bool {
	return len(config.Values) > 0 ||
		len(config.Accelerators) > 0 ||
		len(config.Cookies.Allow) > 0
}

func (config PreferenceDefaultsConfig) HasLocalStateDefaults() bool {
	return len(config.LocalStateValues) > 0
}

func (config PreferenceDefaultsConfig) HasVariationDefaults() bool {
	return len(config.VariationValues) > 0
}

func (config PreferenceDefaultsConfig) Patch(preferences map[string]any) {
	for _, value := range config.Values {
		SetNestedValue(preferences, value.Path, value.Value)
	}
	for _, accelerator := range config.Accelerators {
		customAccelerators := NestedObject(preferences, accelerator.Path)
		EnsureAcceleratorAdded(customAccelerators, accelerator.CommandID, accelerator.Accelerator)
	}
	SetCookieAllowlist(preferences, config.Cookies.Allow)
}

func (config PreferenceDefaultsConfig) PatchLocalState(localState map[string]any) {
	for _, value := range config.LocalStateValues {
		SetNestedValue(localState, value.Path, value.Value)
	}
}

func (config PreferenceDefaultsConfig) PatchVariations(variations map[string]any) {
	for _, value := range config.VariationValues {
		variations[value.Path] = value.Value
	}
}

func expandPathTemplate(path string) string {
	if path == "" {
		return ""
	}
	home := homeDir()
	variables := map[string]string{
		homeTemplateVariable:       home,
		configHomeTemplateVariable: userdirs.ConfigHome(home),
		dataHomeTemplateVariable:   userdirs.DataHome(home),
	}
	return filepath.FromSlash(os.Expand(path, func(name string) string {
		if value, ok := variables[name]; ok {
			return value
		}
		return "${" + name + "}"
	}))
}
