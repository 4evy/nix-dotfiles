package extensions

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4evy/dotfiles/internal/common/chromiumext"
	"github.com/4evy/dotfiles/internal/common/fileutil"
)

func TestValidExtensionIDMatchesChromiumIDShape(t *testing.T) {
	for _, id := range []string{
		"aeblfdkhhhdcdjpifhhbdiojplfjncoa",
		"lkbebcjgcmobigpeffafkodonchffocl",
	} {
		if !chromiumext.ValidExtensionID(id) {
			t.Fatalf("validExtensionID(%q) = false", id)
		}
	}

	for _, id := range []string{
		"",
		"aeblfdkhhhdcdjpifhhbdiojplfjnco",
		"aeblfdkhhhdcdjpifhhbdiojplfjncoq",
		"AEBLFDKHHHDCDJPIFHHBDIOJPLFJNCOA",
	} {
		if chromiumext.ValidExtensionID(id) {
			t.Fatalf("validExtensionID(%q) = true", id)
		}
	}
}

func TestValidExternalVersionMatchesChromiumExternalVersionShape(t *testing.T) {
	for _, version := range []string{"1", "1.2", "8.12.24.34"} {
		if !chromiumext.ValidExternalVersion(version) {
			t.Fatalf("validExternalVersion(%q) = false", version)
		}
	}

	for _, version := range []string{"", "1.", ".1", "1.beta", "1..2"} {
		if chromiumext.ValidExternalVersion(version) {
			t.Fatalf("validExternalVersion(%q) = true", version)
		}
	}
}

func TestChromeStoreCRXDownloadURL(t *testing.T) {
	got, err := chromiumext.ChromeStoreCRXDownloadURL(
		"https://clients2.google.com/service/update2/crx",
		"aeblfdkhhhdcdjpifhhbdiojplfjncoa",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "https://clients2.google.com/service/update2/crx?") {
		t.Fatalf("download URL = %q, want Chrome update endpoint", got)
	}
	for _, want := range []string{
		"response=redirect",
		"prodversion=150.0.0.0",
		"acceptformat=crx2%2Ccrx3",
		"x=id%3Daeblfdkhhhdcdjpifhhbdiojplfjncoa%26uc",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("download URL = %q, missing %q", got, want)
		}
	}
}

func TestInstallPinsChromeStoreExtensionsToCRXFiles(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	_, err := Install(Options{
		Root:         filepath.Join(root, "cache"),
		ExternalDirs: []string{filepath.Join(home, ".config/net.imput.helium/External Extensions")},
		ExcludedIDs: map[string]bool{
			"cjpalhdlnbpafiamejdnhcphjbkeiagm": true,
		},
		Download: func(path, url string) error {
			if err := os.MkdirAll(filepath.Dir(path), fileutil.DefaultDirPerm); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(url), fileutil.DefaultFilePerm)
		},
		Resolve: func(rawURL string) (string, error) {
			parsed, err := url.Parse(rawURL)
			if err != nil {
				return "", err
			}
			rawX := parsed.Query().Get("x")
			id, _, ok := strings.Cut(strings.TrimPrefix(rawX, "id="), "&")
			if !ok {
				t.Fatalf("unexpected Chrome Store x query = %q", rawX)
			}
			return "https://clients2.googleusercontent.com/crx/blobs/example/" +
				strings.ToUpper(id) + "_8_12_24_34.crx", nil
		},
		Verify:    func(string, string) error { return nil },
		VerifyCRX: func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	externalDir := filepath.Join(home, ".config/net.imput.helium/External Extensions")
	store := readExternalJSONTest(
		t,
		filepath.Join(externalDir, "aeblfdkhhhdcdjpifhhbdiojplfjncoa.json"),
	)
	if got := store["external_crx"]; got == "" {
		t.Fatalf("external_crx is empty: %#v", store)
	}
	if got := store["external_version"]; got != "8.12.24.34" {
		t.Fatalf("external_version = %q, want resolved Chrome Store CRX version", got)
	}
	if _, ok := store["external_update_url"]; ok {
		t.Fatalf("Chrome Store extension should be pinned to external_crx: %#v", store)
	}

	crx := readExternalJSONTest(
		t,
		filepath.Join(externalDir, "bolggfoncklhniejomgplkjcllmnonbh.json"),
	)
	if crx["external_crx"] == "" || crx["external_version"] == "" {
		t.Fatalf("non-store CRX extension should remain pinned by file/version: %#v", crx)
	}

	bpc := readExternalJSONTest(
		t,
		filepath.Join(externalDir, "lkbebcjgcmobigpeffafkodonchffocl.json"),
	)
	if got := bpc["external_update_url"]; got != "https://gitflic.ru/project/magnolia1234/bpc_updates/blob/raw?file=updates.xml" {
		t.Fatalf("BPC update URL = %q, want upstream update feed", got)
	}
	if _, ok := bpc["external_crx"]; ok {
		t.Fatalf("BPC should use update URL instead of pinned CRX: %#v", bpc)
	}
}

func TestCatalogGetsLatestUBlockOriginFromGitHub(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, extension := range catalog.ZIP {
		if extension.ID != "cjpalhdlnbpafiamejdnhcphjbkeiagm" {
			continue
		}
		if extension.Repository != "gorhill/uBlock" {
			t.Fatalf("uBlock Origin repository = %q", extension.Repository)
		}
		if extension.AssetTemplate != "uBlock0_{tag}.chromium.zip" {
			t.Fatalf("uBlock Origin asset template = %q", extension.AssetTemplate)
		}
		if !extension.LoadUnpacked || extension.ArchiveRoot != "uBlock0.chromium" {
			t.Fatalf("uBlock Origin unpacked metadata = %#v", extension)
		}
		return
	}
	t.Fatal("latest uBlock Origin GitHub release is missing from the Chromium catalog")
}

func readExternalJSONTest(t *testing.T, path string) map[string]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value := map[string]string{}
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
