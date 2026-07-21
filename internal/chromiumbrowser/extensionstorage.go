package chromiumbrowser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"syscall"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	lzstring "github.com/daku10/go-lz-string"
	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

const (
	localStorageArea           = "local"
	syncStorageArea            = "sync"
	localExtensionSettingsDir  = "Local Extension Settings"
	syncExtensionSettingsDir   = "Sync Extension Settings"
	jsonStorageEncoding        = "json"
	lzStringURIStorageEncoding = "json-lz-string-uri"
)

type settingsFile struct {
	Local       []extensionSettings `json:"local"`
	Sync        []extensionSettings `json:"sync"`
	LocalAppend []extensionSettings `json:"local_append"`
	SyncAppend  []extensionSettings `json:"sync_append"`
	Inputs      []extensionInput    `json:"inputs"`
}

type extensionSettings struct {
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
}

type extensionInput struct {
	Name     string `json:"name"`
	Area     string `json:"area"`
	ID       string `json:"id"`
	Key      string `json:"key"`
	Path     string `json:"path"`
	Encoding string `json:"encoding"`
}

func ApplyExtensionSettings(options ApplyOptions) error {
	sources := slices.Clone(options.SettingsSource)
	for _, path := range options.Settings {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read settings file %s: %w", path, err)
		}
		sources = append(sources, SettingsSource{Name: path, Data: data})
	}

	for _, source := range sources {
		var settings settingsFile
		if err := decodeJSON(bytes.NewReader(source.Data), &settings); err != nil {
			return fmt.Errorf("parse settings file %s: %w", source.Name, err)
		}
		for _, entry := range settings.Local {
			entry.ID = resolveExtensionID(options.ExtensionIDAliases, entry.ID)
			if err := writeStorageValues(
				options.ProfileDir,
				localExtensionSettingsDir,
				entry,
			); err != nil {
				return err
			}
		}
		for _, entry := range settings.Sync {
			entry.ID = resolveExtensionID(options.ExtensionIDAliases, entry.ID)
			if err := writeStorageValues(
				options.ProfileDir,
				syncExtensionSettingsDir,
				entry,
			); err != nil {
				return err
			}
		}
		for _, entry := range settings.LocalAppend {
			entry.ID = resolveExtensionID(options.ExtensionIDAliases, entry.ID)
			if err := appendStorageValues(
				options.ProfileDir,
				localExtensionSettingsDir,
				entry,
			); err != nil {
				return err
			}
		}
		for _, entry := range settings.SyncAppend {
			entry.ID = resolveExtensionID(options.ExtensionIDAliases, entry.ID)
			if err := appendStorageValues(
				options.ProfileDir,
				syncExtensionSettingsDir,
				entry,
			); err != nil {
				return err
			}
		}
		for _, input := range settings.Inputs {
			input.ID = resolveExtensionID(options.ExtensionIDAliases, input.ID)
			value, ok := options.Input.ExtensionValues[input.Name]
			if !ok || value == "" {
				continue
			}
			if err := applyExtensionInput(options.ProfileDir, input, value); err != nil {
				return fmt.Errorf("apply input %q from %s: %w", input.Name, source.Name, err)
			}
		}
	}
	return nil
}

func resolveExtensionID(aliases map[string]string, id string) string {
	if alias := aliases[id]; alias != "" {
		return alias
	}
	return id
}

func applyExtensionInput(profileDir string, input extensionInput, value string) error {
	area, err := extensionStorageArea(input.Area)
	if err != nil {
		return err
	}
	if input.ID == "" || input.Key == "" || input.Path == "" {
		return errors.New("id, key, and path are required")
	}
	return withStorage(profileDir, area, input.ID, func(db *leveldb.DB) error {
		document := map[string]any{}
		raw, err := db.Get([]byte(input.Key), nil)
		if err == nil {
			document, err = decodeStorageObject(raw, input.Encoding)
			if err != nil {
				return fmt.Errorf("decode %s/%s/%s: %w", area, input.ID, input.Key, err)
			}
		} else if !errors.Is(err, leveldb.ErrNotFound) {
			return fmt.Errorf("read %s/%s/%s: %w", area, input.ID, input.Key, err)
		}
		SetNestedValue(document, input.Path, value)
		stored, err := encodeStorageObject(document, input.Encoding)
		if err != nil {
			return fmt.Errorf("encode %s/%s/%s: %w", area, input.ID, input.Key, err)
		}
		return db.Put([]byte(input.Key), stored, nil)
	})
}

func extensionStorageArea(area string) (string, error) {
	switch area {
	case localStorageArea:
		return localExtensionSettingsDir, nil
	case syncStorageArea:
		return syncExtensionSettingsDir, nil
	default:
		return "", fmt.Errorf("unsupported storage area %q", area)
	}
}

func decodeStorageObject(raw []byte, encoding string) (map[string]any, error) {
	if encoding == lzStringURIStorageEncoding {
		var compressed string
		if err := json.Unmarshal(raw, &compressed); err != nil {
			return nil, err
		}
		decoded, err := lzstring.DecompressFromEncodedURIComponent(compressed)
		if err != nil {
			return nil, err
		}
		raw = []byte(decoded)
	} else if encoding != jsonStorageEncoding {
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	document := map[string]any{}
	if err := decodeJSON(bytes.NewReader(raw), &document); err != nil {
		return nil, err
	}
	return document, nil
}

func encodeStorageObject(document map[string]any, encoding string) ([]byte, error) {
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	if encoding == jsonStorageEncoding {
		return encoded, nil
	}
	if encoding != lzStringURIStorageEncoding {
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
	compressed, err := lzstring.CompressToEncodedURIComponent(string(encoded))
	if err != nil {
		return nil, err
	}
	return json.Marshal(compressed)
}

func writeStorageValues(profileDir, area string, entry extensionSettings) error {
	return withStorage(profileDir, area, entry.ID, func(db *leveldb.DB) error {
		batch := new(leveldb.Batch)
		for key, value := range entry.Values {
			encoded, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("encode %s/%s/%s: %w", area, entry.ID, key, err)
			}
			batch.Put([]byte(key), encoded)
		}
		return db.Write(batch, nil)
	})
}

func appendStorageValues(profileDir, area string, entry extensionSettings) error {
	return withStorage(profileDir, area, entry.ID, func(db *leveldb.DB) error {
		batch := new(leveldb.Batch)
		for key, value := range entry.Values {
			additions, ok := value.([]any)
			if !ok {
				return fmt.Errorf("append %s/%s/%s: value must be an array", area, entry.ID, key)
			}
			existing := []any{}
			raw, err := db.Get([]byte(key), nil)
			if err == nil {
				if err := decodeJSON(bytes.NewReader(raw), &existing); err != nil {
					return fmt.Errorf("decode %s/%s/%s for append: %w", area, entry.ID, key, err)
				}
			} else if !errors.Is(err, leveldb.ErrNotFound) {
				return fmt.Errorf("read %s/%s/%s for append: %w", area, entry.ID, key, err)
			}
			for _, addition := range additions {
				if !slices.ContainsFunc(existing, func(current any) bool {
					return reflect.DeepEqual(current, addition)
				}) {
					existing = append(existing, addition)
				}
			}
			encoded, err := json.Marshal(existing)
			if err != nil {
				return fmt.Errorf("encode %s/%s/%s after append: %w", area, entry.ID, key, err)
			}
			batch.Put([]byte(key), encoded)
		}
		return db.Write(batch, nil)
	})
}

func withStorage(
	profileDir,
	area,
	extensionID string,
	operation func(*leveldb.DB) error,
) (err error) {
	path := filepath.Join(profileDir, area, extensionID)
	if err := os.MkdirAll(path, fileutil.DefaultDirPerm); err != nil {
		return fmt.Errorf("create storage directory %s: %w", path, err)
	}
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return fmt.Errorf("open storage %s: %w", path, err)
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	return operation(db)
}

func isStorageTemporarilyUnavailable(err error) bool {
	if errors.Is(err, storage.ErrLocked) || errors.Is(err, syscall.EAGAIN) {
		return true
	}
	var corrupted *leveldberrors.ErrCorrupted
	if !errors.As(err, &corrupted) {
		return false
	}
	var missing *leveldberrors.ErrMissingFiles
	return errors.As(corrupted.Err, &missing)
}
