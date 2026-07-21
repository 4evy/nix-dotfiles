package themerunner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	homeShorthand       = "~"
	homeShorthandPrefix = "~/"
)

func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("HOME is not set")
	}
	return home, nil
}

func expandPath(path string) string {
	if path == "" {
		return ""
	}
	home, err := homeDir()
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
