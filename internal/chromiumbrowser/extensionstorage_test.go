package chromiumbrowser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	lzstring "github.com/daku10/go-lz-string"
	"github.com/syndtr/goleveldb/leveldb"
	"gotest.tools/v3/assert"
)

func TestApplyWritesLocalAndSyncSettings(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	profileDir := filepath.Join(root, "profile")
	settingsJSON := `{
		"local": [
			{
				"id": "local-extension",
				"values": {
					"enabled": true,
					"count": 2,
					"nested": {"mode": "quiet"}
				}
			}
		],
		"sync": [
			{
				"id": "sync-extension",
				"values": {
					"name": "Chromium"
				}
			}
		]
	}`
	if err := os.WriteFile(settingsPath, []byte(settingsJSON), testPrivatePerm); err != nil {
		t.Fatal(err)
	}

	if err := ApplyExtensionSettings(
		ApplyOptions{ProfileDir: profileDir, Settings: []string{settingsPath}},
	); err != nil {
		t.Fatal(err)
	}

	assertStoredValue(
		t,
		profileDir,
		"Local Extension Settings",
		"local-extension",
		"enabled",
		"true",
	)
	assertStoredValue(t, profileDir, "Local Extension Settings", "local-extension", "count", "2")
	assertStoredValue(
		t,
		profileDir,
		"Local Extension Settings",
		"local-extension",
		"nested",
		`{"mode":"quiet"}`,
	)
	assertStoredValue(
		t,
		profileDir,
		"Sync Extension Settings",
		"sync-extension",
		"name",
		`"Chromium"`,
	)
}

func TestBrowserApplyExtensionSettingsIncludesEmbeddedDefaults(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "profile")
	browser := Config{ExecutableName: testExecutableName}.Browser()
	if err := browser.ApplyExtensionSettings(ApplyOptions{ProfileDir: profileDir}); err != nil {
		t.Fatal(err)
	}
	sources, err := DefaultSettingsSources()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		var settings settingsFile
		if err := json.Unmarshal(source.Data, &settings); err != nil {
			t.Fatal(err)
		}
		for _, entry := range settings.Local {
			for key, value := range entry.Values {
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				assertStoredValue(
					t,
					profileDir,
					"Local Extension Settings",
					entry.ID,
					key,
					string(encoded),
				)
			}
		}
	}
}

func TestBrowserApplyExtensionSettingsIncludesBrowserDefaults(t *testing.T) {
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	profileDir := filepath.Join(t.TempDir(), "profile")
	browser := Config{ExecutableName: testExecutableName}.Browser()
	browser.DefaultSettings = []SettingsSource{{
		Name: "test browser defaults",
		Data: []byte(`{
			"local": [{
				"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"values": {
					"browserDefault": true
				}
			}],
			"sync": []
		}`),
	}}
	if err := browser.ApplyExtensionSettings(ApplyOptions{ProfileDir: profileDir}); err != nil {
		t.Fatal(err)
	}

	assertStoredValue(
		t,
		profileDir,
		"Local Extension Settings",
		extensionID,
		"browserDefault",
		"true",
	)
}

func TestApplyExtensionInputMergesEncodedObject(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "profile")
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	dbPath := filepath.Join(profileDir, "Sync Extension Settings", extensionID)
	if err := os.MkdirAll(dbPath, testDirPerm); err != nil {
		t.Fatal(err)
	}
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(map[string]any{
		"theme": "dark",
		"credentials": map[string]any{
			"token": "old-token",
		},
	})
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	compressed, err := lzstring.CompressToEncodedURIComponent(string(encoded))
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	stored, err := json.Marshal(compressed)
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Put([]byte("options"), stored, nil); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	err = ApplyExtensionSettings(ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "input test",
			Data: []byte(`{
				"inputs": [{
					"name": "access-token",
					"area": "sync",
					"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"key": "options",
					"path": "credentials.token",
					"encoding": "json-lz-string-uri"
				}]
			}`),
		}},
		Input: ApplyInput{
			ExtensionValues: map[string]string{
				"access-token": "new-token",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	db, err = leveldb.OpenFile(dbPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close extension settings: %v", err)
		}
	})

	raw, err := db.Get([]byte("options"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var compressedOptions string
	if err := json.Unmarshal(raw, &compressedOptions); err != nil {
		t.Fatal(err)
	}
	decompressed, err := lzstring.DecompressFromEncodedURIComponent(compressedOptions)
	if err != nil {
		t.Fatal(err)
	}
	var options map[string]any
	if err := json.Unmarshal([]byte(decompressed), &options); err != nil {
		t.Fatal(err)
	}
	if got := options["theme"]; got != "dark" {
		t.Fatalf("theme = %v, want dark", got)
	}
	credentials := options["credentials"].(map[string]any)
	if got := credentials["token"]; got != "new-token" {
		t.Fatalf("credentials.token = %v, want new-token", got)
	}
}

func TestDefaultSettingsSourcesAreValid(t *testing.T) {
	sources, err := DefaultSettingsSources()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		var settings any
		if err := json.Unmarshal(source.Data, &settings); err != nil {
			t.Fatalf("%s: %v", source.Name, err)
		}
	}
}

func TestAppendSettingsUseExtensionAliasAndDoNotDuplicateValues(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "profile")
	const sourceID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const targetID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	err := ApplyExtensionSettings(ApplyOptions{
		ProfileDir:         profileDir,
		ExtensionIDAliases: map[string]string{sourceID: targetID},
		SettingsSource: []SettingsSource{
			{
				Name: "base",
				Data: []byte(`{
					"local": [{
						"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						"values": {"items": ["base", "shared"]}
					}]
				}`),
			},
			{
				Name: "overlay",
				Data: []byte(`{
					"local_append": [{
						"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						"values": {"items": ["shared", "overlay"]}
					}]
				}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(
		t,
		storedStringList(t, profileDir, "Local Extension Settings", targetID, "items"),
		[]string{"base", "shared", "overlay"},
	)
	if _, err := os.Stat(filepath.Join(profileDir, "Local Extension Settings", sourceID)); !os.IsNotExist(err) {
		t.Fatalf("source extension storage exists after aliasing: %v", err)
	}
}

func assertStoredValue(t *testing.T, profileDir, area, extensionID, key, want string) {
	t.Helper()
	db, err := leveldb.OpenFile(filepath.Join(profileDir, area, extensionID), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close extension settings: %v", err)
		}
	}()

	got, err := db.Get([]byte(key), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s/%s/%s = %s, want %s", area, extensionID, key, got, want)
	}
}

func storedStringList(t *testing.T, profileDir, area, extensionID, key string) []string {
	t.Helper()
	db, err := leveldb.OpenFile(filepath.Join(profileDir, area, extensionID), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close extension settings: %v", err)
		}
	}()
	raw, err := db.Get([]byte(key), nil)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	return values
}
