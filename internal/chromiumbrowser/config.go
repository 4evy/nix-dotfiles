package chromiumbrowser

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/dotfiles/internal/common/userdirs"
)

type Config struct {
	Name           string                   `toml:"name"`
	LogPrefix      string                   `toml:"log_prefix"`
	ExecutableName string                   `toml:"executable_name"`
	AliasName      string                   `toml:"alias_name"`
	Linux          LinuxConfig              `toml:"linux"`
	MacOS          MacOSConfig              `toml:"macos"`
	Paths          map[string]ModePaths     `toml:"paths"`
	Preferences    PreferenceDefaultsConfig `toml:"preferences"`
	ExtensionIDs   ExtensionIDs             `toml:"extensions"`
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
	browser := Browser{
		Name:              config.Name,
		LogPrefix:         config.LogPrefix,
		ExecutableName:    config.ExecutableName,
		AliasName:         config.AliasName,
		LinuxDesktopID:    config.Linux.DesktopID,
		LinuxWrapperFlags: slices.Clone(config.Linux.WrapperFlags),
		LinuxLauncherName: config.Linux.LauncherName,
		LinuxDesktopName:  config.Linux.DesktopName,
		LinuxDesktopExec:  config.Linux.DesktopExec,
		LinuxIconName:     config.Linux.IconName,
		LinuxIconSource:   config.Linux.IconSource,
		MacOSAppDir:       expandPathTemplate(config.MacOS.AppDir),
		MacOSLauncherPath: filepath.FromSlash(config.MacOS.LauncherPath),
		ExternalDirs:      config.ExternalExtensionDirs,
		DefaultProfileDir: config.DefaultProfileDir,
		ExtensionIDs:      config.ExtensionIDs,
	}
	if config.Preferences.HasDefaults() {
		browser.PreferencePatches = []PreferencePatch{config.Preferences.Patch}
	}
	if config.Preferences.HasLocalStateDefaults() {
		browser.LocalStatePatches = []PreferencePatch{config.Preferences.PatchLocalState}
	}
	if config.Preferences.HasVariationDefaults() {
		browser.VariationPatches = []PreferencePatch{config.Preferences.PatchVariations}
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
	return filepath.FromSlash(strings.NewReplacer(
		"${home}", home,
		"${config_home}", userdirs.ConfigHome(home),
		"${data_home}", userdirs.DataHome(home),
	).Replace(path))
}
