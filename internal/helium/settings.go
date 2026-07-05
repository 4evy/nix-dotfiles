package helium

import (
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/4evy/dotfiles/internal/chromiumbrowser"
	"github.com/buildkite/shellwords"
)

type (
	ApplyOptions   = chromiumbrowser.ApplyOptions
	SettingsSource = chromiumbrowser.SettingsSource
)

const defaultSettingsDir = "settings/default"

//go:embed settings/default/*.json
var defaultSettingsFS embed.FS

func DefaultSettingsSources() ([]SettingsSource, error) {
	entries, err := fs.ReadDir(defaultSettingsFS, defaultSettingsDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded default Helium extension settings: %w", err)
	}
	sources := make([]SettingsSource, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := defaultSettingsDir + "/" + entry.Name()
		data, err := defaultSettingsFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf(
				"read embedded default Helium extension settings file %s: %w",
				path,
				err,
			)
		}
		sources = append(sources, SettingsSource{Name: "embedded " + path, Data: data})
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("embedded default Helium extension settings are empty")
	}
	return sources, nil
}

func ApplyExtensionSettings(options ApplyOptions) error {
	options.ExtensionIDs = options.ExtensionIDs.WithFallback(defaultConfig.Browser.ExtensionIDs)
	return chromiumbrowser.ApplyExtensionSettings(options)
}

func applyThemePreferencesFromFlags(preferences map[string]any, flags string) {
	userColor, ok := userColorFromFlags(flags)
	if !ok {
		return
	}
	theme := chromiumbrowser.NestedObject(preferences, "browser.theme")
	theme["color_variant2"] = 1
	theme["is_grayscale2"] = false
	theme["user_color2"] = userColor

	extensionTheme := chromiumbrowser.NestedObject(preferences, "extensions.theme")
	extensionTheme["id"] = "user_color_theme_id"
}

func userColorFromFlags(flags string) (int64, bool) {
	fields, err := shellwords.Split(flags)
	if err != nil {
		return 0, false
	}
	for _, field := range fields {
		value, ok := strings.CutPrefix(field, "--set-user-color=")
		if !ok {
			continue
		}
		parts := strings.Split(value, ",")
		if len(parts) != 3 {
			return 0, false
		}
		var rgb [3]int64
		for i, part := range parts {
			parsed, err := strconv.ParseInt(part, 10, 64)
			if err != nil || parsed < 0 || parsed > 255 {
				return 0, false
			}
			rgb[i] = parsed
		}
		argb := int64(0xff000000 | rgb[0]<<16 | rgb[1]<<8 | rgb[2])
		if argb >= 1<<31 {
			argb -= 1 << 32
		}
		return argb, true
	}
	return 0, false
}
