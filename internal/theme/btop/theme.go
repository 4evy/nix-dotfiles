package btop

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	"github.com/4evy/dotfiles/internal/common/userdirs"
	"github.com/4evy/dotfiles/internal/theme/terminal"
	"gopkg.in/ini.v1"
)

const (
	longConfigArg          = "--config"
	shortConfigArg         = "-c"
	defaultConfigPath      = ".config/btop/btop.conf"
	themeCacheDir          = "terminal-theme-run/btop"
	temporaryConfigPattern = "terminal-theme-run-btop-*.conf"
	darkThemeName          = "catppuccin_frappe_pink"
	lightThemeName         = "catppuccin_latte_pink"
	colorThemeKey          = "color_theme"
	homeShorthand          = "~"
	homeShorthandPrefix    = "~/"
	flagValueSeparator     = "="
	configArgPairSize      = 2
)

var configArgNames = []string{longConfigArg, shortConfigArg}

// ThemeArgs replaces btop's config argument with a temporary themed config.
func ThemeArgs(mode terminal.Mode, extraArgs []string) ([]string, func(), error) {
	configText := ""
	if data, err := os.ReadFile(configPath(extraArgs)); err == nil {
		configText = string(data)
	}
	patched, err := patchConfig(configText, mode)
	if err != nil {
		return nil, nil, err
	}

	cache := filepath.Join(userdirs.CacheHome(""), themeCacheDir)
	if err := os.MkdirAll(cache, fileutil.DefaultDirPerm); err != nil {
		return nil, nil, err
	}
	file, err := os.CreateTemp(cache, temporaryConfigPattern)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if _, err := file.WriteString(patched); err != nil {
		closeErr := file.Close()
		cleanup()
		return nil, nil, errors.Join(err, closeErr)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}

	args := []string{longConfigArg, file.Name()}
	return append(args, stripConfigArgs(extraArgs)...), cleanup, nil
}

func configPath(args []string) string {
	for index, arg := range args {
		for _, name := range configArgNames {
			if arg == name && index+1 < len(args) {
				return expandPath(args[index+1])
			}
			if value, ok := strings.CutPrefix(arg, name+flagValueSeparator); ok {
				return expandPath(value)
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, defaultConfigPath)
}

func expandPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == homeShorthand {
		return home
	}
	if rest, ok := strings.CutPrefix(path, homeShorthandPrefix); ok {
		return filepath.Join(home, rest)
	}
	return path
}

func stripConfigArgs(args []string) []string {
	out := slices.Clone(args)
	for index := 0; index < len(out); {
		arg := out[index]
		if slices.Contains(configArgNames, arg) {
			end := min(index+configArgPairSize, len(out))
			out = slices.Delete(out, index, end)
			continue
		}
		if slices.ContainsFunc(configArgNames, func(name string) bool {
			return strings.HasPrefix(arg, name+flagValueSeparator)
		}) {
			out = slices.Delete(out, index, index+1)
			continue
		}
		index++
	}
	return out
}

func patchConfig(configText string, mode terminal.Mode) (string, error) {
	cfg, err := ini.LoadSources(ini.LoadOptions{
		AllowBooleanKeys:        true,
		Insensitive:             false,
		InsensitiveSections:     false,
		InsensitiveKeys:         false,
		PreserveSurroundedQuote: true,
		SkipUnrecognizableLines: true,
		IgnoreInlineComment:     true,
	}, []byte(configText))
	if err != nil {
		return "", err
	}
	theme := darkThemeName
	if mode == terminal.Light {
		theme = lightThemeName
	}
	cfg.Section(ini.DefaultSection).Key(colorThemeKey).SetValue(strconv.Quote(theme))

	var out bytes.Buffer
	if _, err := cfg.WriteTo(&out); err != nil {
		return "", err
	}
	return out.String(), nil
}
