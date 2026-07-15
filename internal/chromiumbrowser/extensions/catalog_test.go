package extensions

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4evy/dotfiles/internal/common/chromiumext"
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

func TestUnpackedExtensionIDMatchesChromiumPathID(t *testing.T) {
	path := "/var/home/4evy/.cache/dotfiles/ansible/helium-browser/extensions/unpacked/aeblfdkhhhdcdjpifhhbdiojplfjncoa"
	want := "eemjflinlmihebpkelplffenpkclceef"
	if got := chromiumext.UnpackedExtensionID(path); got != want {
		t.Fatalf("UnpackedExtensionID(%q) = %q, want %q", path, got, want)
	}
	if got := chromiumext.UnpackedExtensionID(path + "/../aeblfdkhhhdcdjpifhhbdiojplfjncoa"); got != want {
		t.Fatalf("UnpackedExtensionID with redundant path segments = %q, want %q", got, want)
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
		"prodversion=140.0.0.0",
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

	result, err := Install(Options{
		Root:         filepath.Join(root, "cache"),
		ExternalDirs: []string{filepath.Join(home, ".config/net.imput.helium/External Extensions")},
		Download: func(path, url string) error {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(url), 0o644)
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
		Unzip: func(zipPath, dst string) error {
			bundles := filepath.Join(dst, "bundles")
			if err := os.MkdirAll(bundles, 0o755); err != nil {
				return err
			}
			return os.WriteFile(
				filepath.Join(bundles, "common-background.bundle.js"),
				[]byte(`case"install":yield browser.runtime.openOptionsPage();break;case"update":`),
				0o644,
			)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.LoadExtensionPaths) != 0 {
		t.Fatalf("load extension paths = %#v, want none", result.LoadExtensionPaths)
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

func TestPatchUnpackedExtensionSuppressesInstallOptionsPage(t *testing.T) {
	root := t.TempDir()
	bundles := filepath.Join(root, "bundles")
	if err := os.MkdirAll(bundles, 0o755); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(bundles, "common-background.bundle.js")
	input := `case"install":yield browser.runtime.openOptionsPage();break;case"update":`
	if err := os.WriteFile(bundle, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := chromiumext.PatchUnpackedExtension(
		root,
		[]chromiumext.BundlePatch{SuppressInstallOptionsPagePatch},
	); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "openOptionsPage") {
		t.Fatalf("patched bundle still opens options page: %q", text)
	}
	if want := `case"install":break;case"update":`; !strings.Contains(text, want) {
		t.Fatalf("patched bundle = %q, want %q", text, want)
	}
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
