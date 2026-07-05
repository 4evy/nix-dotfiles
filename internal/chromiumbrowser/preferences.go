package chromiumbrowser

import (
	"bytes"
	"encoding/json"
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
)

type PreferencePatch func(map[string]any)

func (browser Browser) ApplyBrowserPreferenceSettings(profileDir string) error {
	preferences, err := ReadPreferences(profileDir)
	if err != nil {
		return err
	}

	for _, patch := range browser.PreferencePatches {
		patch(preferences)
	}

	return WritePreferences(profileDir, preferences)
}

func (browser Browser) ApplyBrowserLocalStateSettings(profileDir string) error {
	if len(browser.LocalStatePatches) == 0 {
		return nil
	}
	localState, err := ReadLocalState(profileDir)
	if err != nil {
		return err
	}
	for _, patch := range browser.LocalStatePatches {
		patch(localState)
	}
	return WriteLocalState(profileDir, localState)
}

func (browser Browser) ApplyBrowserVariationSettings(profileDir string) error {
	if len(browser.VariationPatches) == 0 {
		return nil
	}
	variations, err := ReadVariations(profileDir)
	if err != nil {
		return err
	}
	for _, patch := range browser.VariationPatches {
		patch(variations)
	}
	return WriteVariations(profileDir, variations)
}

func ReadPreferences(profileDir string) (map[string]any, error) {
	path := filepath.Join(profileDir, PreferencesFilename)
	preferences, err := readPreferenceFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Chromium Preferences: %w", err)
	}
	return preferences, nil
}

func WritePreferences(profileDir string, preferences map[string]any) error {
	if _, err := fileutil.WriteJSONIfChanged(
		filepath.Join(profileDir, PreferencesFilename),
		preferences,
		0o600,
	); err != nil {
		return fmt.Errorf("write Chromium Preferences: %w", err)
	}
	return nil
}

func ReadLocalState(profileDir string) (map[string]any, error) {
	localState, err := readPreferenceFile(localStatePath(profileDir))
	if err != nil {
		return nil, fmt.Errorf("read Chromium Local State: %w", err)
	}
	return localState, nil
}

func WriteLocalState(profileDir string, localState map[string]any) error {
	if _, err := fileutil.WriteJSONIfChanged(localStatePath(profileDir), localState, 0o600); err != nil {
		return fmt.Errorf("write Chromium Local State: %w", err)
	}
	return nil
}

func ReadVariations(profileDir string) (map[string]any, error) {
	variations, err := readPreferenceFile(variationsPath(profileDir))
	if err != nil {
		return nil, fmt.Errorf("read Chromium Variations: %w", err)
	}
	return variations, nil
}

func WriteVariations(profileDir string, variations map[string]any) error {
	if _, err := fileutil.WriteJSONIfChanged(variationsPath(profileDir), variations, 0o600); err != nil {
		return fmt.Errorf("write Chromium Variations: %w", err)
	}
	return nil
}

func localStatePath(profileDir string) string {
	return filepath.Join(filepath.Dir(profileDir), LocalStateFilename)
}

func variationsPath(profileDir string) string {
	return filepath.Join(filepath.Dir(profileDir), VariationsFilename)
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
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&preferences); err != nil {
		return nil, err
	}
	return preferences, nil
}

func NestedObject(root map[string]any, dottedPath string) map[string]any {
	current := root
	parts := strings.Split(dottedPath, ".")
	for _, part := range parts {
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

	added, ok := command["added"].([]any)
	if !ok {
		added = []any{}
	}
	if slices.ContainsFunc(added, func(existing any) bool {
		return existing == accelerator
	}) {
		command["added"] = added
		return
	}
	command["added"] = append(added, accelerator)
}
