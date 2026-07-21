package chromiumbrowser

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/dotfiles/internal/common/fileutil"
)

const (
	PreferencesFilename = "Preferences"
	LocalStateFilename  = "Local State"
	VariationsFilename  = "Variations"

	acceleratorAddedKey = "added"
)

type PreferencePatch func(map[string]any)

type browserDataFile struct {
	filename    string
	description string
	profileDir  bool
}

var (
	preferencesFile = browserDataFile{
		filename: PreferencesFilename, description: "Chromium Preferences", profileDir: true,
	}
	localStateFile = browserDataFile{
		filename: LocalStateFilename, description: "Chromium Local State",
	}
	variationsFile = browserDataFile{
		filename: VariationsFilename, description: "Chromium Variations",
	}
)

func (browser Browser) ApplyBrowserPreferenceSettings(profileDir string) error {
	return applyBrowserDataSettings(profileDir, preferencesFile, browser.PreferencePatches)
}

func (browser Browser) ApplyBrowserLocalStateSettings(profileDir string) error {
	if len(browser.LocalStatePatches) == 0 {
		return nil
	}
	return applyBrowserDataSettings(profileDir, localStateFile, browser.LocalStatePatches)
}

func (browser Browser) ApplyBrowserVariationSettings(profileDir string) error {
	if len(browser.VariationPatches) == 0 {
		return nil
	}
	return applyBrowserDataSettings(profileDir, variationsFile, browser.VariationPatches)
}

func ReadPreferences(profileDir string) (map[string]any, error) {
	return readBrowserDataFile(profileDir, preferencesFile)
}

func WritePreferences(profileDir string, preferences map[string]any) error {
	return writeBrowserDataFile(profileDir, preferencesFile, preferences)
}

func ReadLocalState(profileDir string) (map[string]any, error) {
	return readBrowserDataFile(profileDir, localStateFile)
}

func WriteLocalState(profileDir string, localState map[string]any) error {
	return writeBrowserDataFile(profileDir, localStateFile, localState)
}

func ReadVariations(profileDir string) (map[string]any, error) {
	return readBrowserDataFile(profileDir, variationsFile)
}

func WriteVariations(profileDir string, variations map[string]any) error {
	return writeBrowserDataFile(profileDir, variationsFile, variations)
}

func applyBrowserDataSettings(
	profileDir string,
	file browserDataFile,
	patches []PreferencePatch,
) error {
	values, err := readBrowserDataFile(profileDir, file)
	if err != nil {
		return err
	}
	for _, patch := range patches {
		patch(values)
	}
	return writeBrowserDataFile(profileDir, file, values)
}

func readBrowserDataFile(profileDir string, file browserDataFile) (map[string]any, error) {
	values, err := readPreferenceFile(file.path(profileDir))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file.description, err)
	}
	return values, nil
}

func writeBrowserDataFile(
	profileDir string,
	file browserDataFile,
	values map[string]any,
) error {
	if _, err := fileutil.WriteJSONIfChanged(
		file.path(profileDir),
		values,
		fileutil.PrivateFilePerm,
	); err != nil {
		return fmt.Errorf("write %s: %w", file.description, err)
	}
	return nil
}

func (file browserDataFile) path(profileDir string) string {
	if file.profileDir {
		return filepath.Join(profileDir, file.filename)
	}
	return filepath.Join(filepath.Dir(profileDir), file.filename)
}

func readPreferenceFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}

	preferences := map[string]any{}
	if err := decodeJSON(bytes.NewReader(data), &preferences); err != nil {
		return nil, err
	}
	return preferences, nil
}

func NestedObject(root map[string]any, dottedPath string) map[string]any {
	current := root
	for part := range strings.SplitSeq(dottedPath, ".") {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	return current
}

func SetNestedValue(root map[string]any, dottedPath string, value any) {
	index := strings.LastIndex(dottedPath, ".")
	if index < 0 {
		root[dottedPath] = value
		return
	}
	parentPath := dottedPath[:index]
	key := dottedPath[index+1:]
	NestedObject(root, parentPath)[key] = value
}

func EnsureAcceleratorAdded(customAccelerators map[string]any, commandID, accelerator string) {
	command, ok := customAccelerators[commandID].(map[string]any)
	if !ok {
		command = map[string]any{}
		customAccelerators[commandID] = command
	}

	added, ok := command[acceleratorAddedKey].([]any)
	if !ok {
		added = []any{}
	}
	if slices.ContainsFunc(added, func(existing any) bool {
		return existing == accelerator
	}) {
		command[acceleratorAddedKey] = added
		return
	}
	command[acceleratorAddedKey] = append(added, accelerator)
}
