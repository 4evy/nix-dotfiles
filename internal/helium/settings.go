package helium

import (
	"embed"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/4evy/dotfiles/internal/chromiumbrowser"
)

const (
	heliumDefaultSettingsPattern       = "settings/default/*.json"
	browserThemePath                   = "browser.theme"
	extensionThemePath                 = "extensions.theme"
	colorVariantKey                    = "color_variant2"
	defaultColorVariant                = 1
	grayscaleKey                       = "is_grayscale2"
	userColorKey                       = "user_color2"
	extensionThemeIDKey                = "id"
	userColorThemeID                   = "user_color_theme_id"
	setUserColorFlagPrefix             = "--set-user-color="
	rgbComponentSeparator              = ","
	rgbComponentCount                  = 3
	redComponentIndex                  = 0
	greenComponentIndex                = 1
	blueComponentIndex                 = 2
	rgbComponentMax                    = 255
	decimalRadix                       = 10
	int64BitSize                       = 64
	redShift                           = 16
	greenShift                         = 8
	opaqueAlphaMask              int64 = 0xff000000
	signedColorBoundary          int64 = 1 << 31
	argbModulus                  int64 = 1 << 32
)

//go:embed settings/default/*.json
var defaultSettingsFS embed.FS

var heliumDefaultSettings = mustDefaultSettingsSources()

func mustDefaultSettingsSources() []chromiumbrowser.SettingsSource {
	sources, err := chromiumbrowser.SettingsSourcesFromFS(
		defaultSettingsFS,
		heliumDefaultSettingsPattern,
	)
	if err != nil {
		panic(fmt.Errorf("load embedded Helium extension settings: %w", err))
	}
	return sources
}

func applyThemePreferencesFromFlags(preferences map[string]any, flags []string) {
	userColor, ok := userColorFromFlags(flags)
	if !ok {
		return
	}
	theme := chromiumbrowser.NestedObject(preferences, browserThemePath)
	theme[colorVariantKey] = defaultColorVariant
	theme[grayscaleKey] = false
	theme[userColorKey] = userColor

	extensionTheme := chromiumbrowser.NestedObject(preferences, extensionThemePath)
	extensionTheme[extensionThemeIDKey] = userColorThemeID
}

func userColorFromFlags(flags []string) (int64, bool) {
	for _, field := range slices.Backward(flags) {
		value, ok := strings.CutPrefix(field, setUserColorFlagPrefix)
		if !ok {
			continue
		}
		parts := strings.Split(value, rgbComponentSeparator)
		if len(parts) != rgbComponentCount {
			return 0, false
		}
		var rgb [rgbComponentCount]int64
		for i, part := range parts {
			parsed, err := strconv.ParseInt(part, decimalRadix, int64BitSize)
			if err != nil || parsed < 0 || parsed > rgbComponentMax {
				return 0, false
			}
			rgb[i] = parsed
		}
		argb := opaqueAlphaMask |
			rgb[redComponentIndex]<<redShift |
			rgb[greenComponentIndex]<<greenShift |
			rgb[blueComponentIndex]
		if argb >= signedColorBoundary {
			argb -= argbModulus
		}
		return argb, true
	}
	return 0, false
}
