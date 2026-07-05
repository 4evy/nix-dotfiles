package helium

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	lzstring "github.com/daku10/go-lz-string"
	"github.com/syndtr/goleveldb/leveldb"
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
					"name": "Helium"
				}
			}
		]
	}`
	if err := os.WriteFile(settingsPath, []byte(settingsJSON), 0o600); err != nil {
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
		`"Helium"`,
	)
}

func TestApplyMergesRefinedGitHubToken(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "profile")
	refinedGitHubID := defaultConfig.Browser.ExtensionIDs.RefinedGitHub
	dbPath := filepath.Join(profileDir, "Sync Extension Settings", refinedGitHubID)
	if err := os.MkdirAll(dbPath, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(map[string]any{
		"theme":         "dark",
		"personalToken": "old-token",
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
		ProfileDir:  profileDir,
		GitHubToken: true,
		TokenFunc: func() string {
			return "new-token"
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	db, err = leveldb.OpenFile(
		filepath.Join(profileDir, "Sync Extension Settings", refinedGitHubID),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

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
	if got := options["personalToken"]; got != "new-token" {
		t.Fatalf("personalToken = %v, want new-token", got)
	}
}

func TestDefaultSettingsSourcesAreValid(t *testing.T) {
	sources, err := DefaultSettingsSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) < 2 {
		t.Fatalf("default settings source count = %d, want multiple files", len(sources))
	}
	totalLocal := 0
	for _, source := range sources {
		var settings struct {
			Local []struct {
				ID     string         `json:"id"`
				Values map[string]any `json:"values"`
			} `json:"local"`
		}
		if err := json.Unmarshal(source.Data, &settings); err != nil {
			t.Fatalf("%s: %v", source.Name, err)
		}
		totalLocal += len(settings.Local)
	}
	if totalLocal == 0 {
		t.Fatal("default settings local entries are empty")
	}
}

func TestUserColorFromFlagsParsesQuotedShellFields(t *testing.T) {
	want, ok := userColorFromFlags("--set-user-color=12,34,56")
	if !ok {
		t.Fatal("plain user color flag was not parsed")
	}

	got, ok := userColorFromFlags(`--some-flag "--set-user-color=12,34,56"`)
	if !ok {
		t.Fatal("quoted user color flag was not parsed")
	}
	if got != want {
		t.Fatalf("quoted user color = %d, want %d", got, want)
	}
}

func assertStoredValue(t *testing.T, profileDir, area, extensionID, key, want string) {
	t.Helper()
	db, err := leveldb.OpenFile(filepath.Join(profileDir, area, extensionID), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	got, err := db.Get([]byte(key), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s/%s/%s = %s, want %s", area, extensionID, key, got, want)
	}
}
