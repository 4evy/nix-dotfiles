package helium

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/4evy/dotfiles/internal/chromiumbrowser"
	"github.com/syndtr/goleveldb/leveldb"
)

func TestDefaultSettingsContainHeliumUBlockComponentFilters(t *testing.T) {
	sources := DefaultBrowser().DefaultSettings
	if len(sources) != 1 {
		t.Fatalf("Helium default settings sources = %d, want 1", len(sources))
	}
	var settings struct {
		LocalAppend []struct {
			ID     string `json:"id"`
			Values struct {
				SelectedFilterLists []string `json:"selectedFilterLists"`
			} `json:"values"`
		} `json:"local_append"`
	}
	if err := json.Unmarshal(sources[0].Data, &settings); err != nil {
		t.Fatalf("%s: %v", sources[0].Name, err)
	}
	if len(settings.LocalAppend) != 1 {
		t.Fatalf("Helium local append settings = %d, want 1", len(settings.LocalAppend))
	}
	if got := settings.LocalAppend[0].ID; got != "cjpalhdlnbpafiamejdnhcphjbkeiagm" {
		t.Fatalf("Helium uBlock base ID = %q", got)
	}
	want := []string{"helium-annoyances", "helium-unbreak"}
	if !slices.Equal(settings.LocalAppend[0].Values.SelectedFilterLists, want) {
		t.Fatalf("Helium uBlock additions = %q, want %q", settings.LocalAppend[0].Values.SelectedFilterLists, want)
	}
}

func TestDefaultBrowserExtendsChromiumUBlockSettings(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	if err := DefaultBrowser().ApplyExtensionSettings(
		chromiumbrowser.ApplyOptions{ProfileDir: profileDir},
	); err != nil {
		t.Fatal(err)
	}

	const componentID = "blockjmkbacgjkknlgpkjjiijinjdanf"
	db, err := leveldb.OpenFile(
		filepath.Join(profileDir, "Local Extension Settings", componentID),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := db.Get([]byte("selectedFilterLists"), nil)
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	var filters []string
	if err := json.Unmarshal(raw, &filters); err != nil {
		t.Fatal(err)
	}
	for _, filter := range []string{"ublock-filters", "helium-annoyances", "helium-unbreak"} {
		if !slices.Contains(filters, filter) {
			t.Errorf("composed Helium uBlock settings are missing %q", filter)
		}
	}
	const webstoreID = "cjpalhdlnbpafiamejdnhcphjbkeiagm"
	if _, err := os.Stat(filepath.Join(profileDir, "Local Extension Settings", webstoreID)); !os.IsNotExist(err) {
		t.Fatalf("web-store uBlock storage exists after Helium ID remap: %v", err)
	}
}

func TestUserColorFromFlagsFindsColorAmongParsedFlags(t *testing.T) {
	got, ok := userColorFromFlags([]string{"--some-flag", "--set-user-color=12,34,56"})
	if !ok {
		t.Fatal("user color flag was not parsed")
	}
	want := int64(0xff0c2238) - 1<<32
	if got != want {
		t.Fatalf("user color = %d, want %d", got, want)
	}
}

func TestUserColorFromFlagsUsesLastValue(t *testing.T) {
	got, ok := userColorFromFlags([]string{
		"--set-user-color=1,2,3",
		"--set-user-color=12,34,56",
	})
	if !ok {
		t.Fatal("user color flag was not parsed")
	}
	want := int64(0xff0c2238) - 1<<32
	if got != want {
		t.Fatalf("user color = %d, want %d", got, want)
	}
}

func TestUserColorFromFlagsRejectsInvalidLastValue(t *testing.T) {
	for _, flags := range [][]string{
		{"--set-user-color=12,34"},
		{"--set-user-color=12,34,256"},
		{"--set-user-color=12,34,pink"},
		{"--set-user-color=1,2,3", "--set-user-color=invalid"},
	} {
		if color, ok := userColorFromFlags(flags); ok {
			t.Fatalf("user color %d parsed from invalid flags %q", color, flags)
		}
	}
}
