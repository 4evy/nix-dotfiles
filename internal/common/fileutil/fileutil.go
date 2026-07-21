package fileutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2/maybe"
	cp "github.com/otiai10/copy"
)

const (
	// DefaultDirPerm is the default permission mode for generated directories.
	DefaultDirPerm fs.FileMode = 0o755
	// DefaultFilePerm is the default permission mode for generated files.
	DefaultFilePerm fs.FileMode = 0o644
	// PrivateFilePerm is the permission mode for generated user-private files.
	PrivateFilePerm fs.FileMode = 0o600
	// ExecutablePerm contains the executable permission bits for all users.
	ExecutablePerm fs.FileMode = 0o111
)

func WriteExecutable(path string, data []byte) error {
	if err := WriteFile(path, data, DefaultFilePerm); err != nil {
		return err
	}
	return MakeExecutable(path)
}

func WriteFile(path string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), DefaultDirPerm); err != nil {
		return err
	}
	return maybe.WriteFile(path, data, perm)
}

func WriteTextIfChanged(path, text string) (bool, error) {
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, []byte(text)) {
		return false, nil
	}
	if err := WriteFile(path, []byte(text), DefaultFilePerm); err != nil {
		return false, err
	}
	return true, nil
}

func WriteJSONIfChanged(path string, value any, perm fs.FileMode) (bool, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return false, err
	}
	data = append(data, '\n')

	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, data) {
		return false, nil
	}
	if err := WriteFile(path, data, perm); err != nil {
		return false, err
	}
	return true, nil
}

func MakeExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.Chmod(path, info.Mode()|ExecutablePerm)
}

func RemoveDirIfExists(path string) error {
	err := os.RemoveAll(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func CopyPath(src, dst string) error {
	return cp.Copy(src, dst, cp.Options{
		OnSymlink: func(string) cp.SymlinkAction {
			return cp.Shallow
		},
	})
}
