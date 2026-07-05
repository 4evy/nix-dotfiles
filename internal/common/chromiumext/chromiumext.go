package chromiumext

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	"github.com/google/go-querystring/query"
	crx3 "github.com/mediabuyerbot/go-crx3"
)

type Catalog struct {
	ChromeStoreUpdateURL string                 `toml:"chrome_store_update_url"`
	ChromeStore          []ChromeStoreExtension `toml:"chrome_store_extensions"`
	UpdateURL            []UpdateURLExtension   `toml:"update_url_extensions"`
	CRX                  []DownloadedExtension  `toml:"crx_extensions"`
	ZIP                  []DownloadedExtension  `toml:"zip_extensions"`
}

type ChromeStoreExtension struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
}

type UpdateURLExtension struct {
	ID        string `toml:"id"`
	Name      string `toml:"name"`
	UpdateURL string `toml:"update_url"`
}

type DownloadedExtension struct {
	ID           string `toml:"id"`
	Name         string `toml:"name"`
	Version      string `toml:"version"`
	URL          string `toml:"url"`
	LoadUnpacked bool   `toml:"load_unpacked"`
}

type Options struct {
	Root          string
	ExternalDirs  []string
	Catalog       Catalog
	Download      func(path, url string) error
	Resolve       func(url string) (string, error)
	Unzip         func(zipPath, dst string) error
	BundlePatches []BundlePatch
}

type Result struct {
	LoadExtensionPaths []string
}

type BundlePatch struct {
	Old string
	New string
}

type chromeStoreCRXQuery struct {
	Response     string `url:"response"`
	ProdVersion  string `url:"prodversion"`
	AcceptFormat string `url:"acceptformat"`
	X            string `url:"x"`
}

type installedCRXExtension struct {
	DownloadedExtension
	Path string
}

type installedUnpackedExtension struct {
	DownloadedExtension
	Path string
}

func Install(options Options) (Result, error) {
	crxDir := filepath.Join(options.Root, "extensions/crx")
	unpackedDir := filepath.Join(options.Root, "extensions/unpacked")
	if err := os.MkdirAll(crxDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(unpackedDir, 0o755); err != nil {
		return Result{}, err
	}

	crxExtensions := make(
		[]installedCRXExtension,
		0,
		len(options.Catalog.CRX)+len(options.Catalog.ChromeStore),
	)
	for _, extension := range options.Catalog.ChromeStore {
		crxURL, err := ChromeStoreCRXDownloadURL(
			options.Catalog.ChromeStoreUpdateURL,
			extension.ID,
		)
		if err != nil {
			return Result{}, err
		}
		resolvedURL := crxURL
		if options.Resolve != nil {
			resolvedURL, err = options.Resolve(crxURL)
			if err != nil {
				return Result{}, err
			}
		}
		version, err := ChromeStoreVersionFromCRXURL(extension.ID, resolvedURL)
		if err != nil {
			return Result{}, err
		}
		crxPath := filepath.Join(crxDir, extension.ID+".crx")
		if err := options.Download(crxPath, crxURL); err != nil {
			return Result{}, err
		}
		if err := verifyCRXID(crxPath, extension.ID); err != nil {
			return Result{}, err
		}
		crxExtensions = append(crxExtensions, installedCRXExtension{
			DownloadedExtension: DownloadedExtension{
				ID:      extension.ID,
				Name:    extension.Name,
				Version: version,
				URL:     crxURL,
			},
			Path: crxPath,
		})
	}
	for _, extension := range options.Catalog.CRX {
		crxPath := filepath.Join(crxDir, extension.ID+".crx")
		if err := options.Download(crxPath, extension.URL); err != nil {
			return Result{}, err
		}
		if err := verifyCRXID(crxPath, extension.ID); err != nil {
			return Result{}, err
		}
		crxExtensions = append(crxExtensions, installedCRXExtension{
			DownloadedExtension: extension,
			Path:                crxPath,
		})
	}

	unpackedExtensions := make([]installedUnpackedExtension, 0, len(options.Catalog.ZIP))
	for _, extension := range options.Catalog.ZIP {
		zipPath := filepath.Join(
			options.Root,
			"extensions",
			extension.ID+"-"+extension.Version+".zip",
		)
		extensionDir := filepath.Join(unpackedDir, extension.ID)
		if err := options.Download(zipPath, extension.URL); err != nil {
			return Result{}, err
		}
		if err := os.RemoveAll(extensionDir); err != nil {
			return Result{}, err
		}
		if err := options.Unzip(zipPath, extensionDir); err != nil {
			return Result{}, err
		}
		if err := PatchUnpackedExtension(extensionDir, options.BundlePatches); err != nil {
			return Result{}, err
		}
		unpackedExtensions = append(unpackedExtensions, installedUnpackedExtension{
			DownloadedExtension: extension,
			Path:                extensionDir,
		})
	}

	for _, externalDir := range options.ExternalDirs {
		if err := os.MkdirAll(externalDir, 0o755); err != nil {
			return Result{}, err
		}
		for _, extension := range crxExtensions {
			if err := writeExternalJSON(
				filepath.Join(externalDir, extension.ID+".json"),
				map[string]string{
					"external_crx":     extension.Path,
					"external_version": extension.Version,
				},
			); err != nil {
				return Result{}, err
			}
		}
		for _, extension := range options.Catalog.UpdateURL {
			if err := writeExternalJSON(
				filepath.Join(externalDir, extension.ID+".json"),
				map[string]string{
					"external_update_url": extension.UpdateURL,
				},
			); err != nil {
				return Result{}, err
			}
		}
	}

	result := Result{}
	for _, extension := range unpackedExtensions {
		if extension.LoadUnpacked {
			result.LoadExtensionPaths = append(result.LoadExtensionPaths, extension.Path)
		}
	}
	return result, nil
}

func ValidateCatalog(catalog Catalog, browser string) error {
	if catalog.ChromeStoreUpdateURL == "" {
		return fmt.Errorf("%s extension catalog is missing chrome_store_update_url", browser)
	}
	if !ValidExternalUpdateURL(catalog.ChromeStoreUpdateURL) {
		return fmt.Errorf("%s extension catalog has an invalid chrome_store_update_url", browser)
	}
	for _, extension := range catalog.ChromeStore {
		if !ValidExtensionID(extension.ID) {
			return fmt.Errorf(
				"%s extension catalog contains a Chrome Store entry with an invalid id",
				browser,
			)
		}
	}
	for _, extension := range catalog.UpdateURL {
		if !ValidExtensionID(extension.ID) || !ValidExternalUpdateURL(extension.UpdateURL) {
			return fmt.Errorf(
				"%s extension catalog contains an incomplete update URL extension entry",
				browser,
			)
		}
	}
	for _, extension := range slices.Concat(catalog.CRX, catalog.ZIP) {
		missingID := extension.ID == ""
		missingVersion := extension.Version == ""
		missingURL := extension.URL == ""
		if missingID || missingVersion || missingURL ||
			!ValidExtensionID(extension.ID) ||
			!ValidExternalVersion(extension.Version) ||
			!ValidExternalUpdateURL(extension.URL) {
			return fmt.Errorf(
				"%s extension catalog contains an incomplete downloaded extension entry",
				browser,
			)
		}
	}
	return nil
}

func ValidExtensionID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, char := range id {
		if char < 'a' || char > 'p' {
			return false
		}
	}
	return true
}

func ValidExternalUpdateURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}

func ValidExternalVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func PatchUnpackedExtension(path string, patches []BundlePatch) error {
	backgroundBundle := filepath.Join(path, "bundles/common-background.bundle.js")
	data, err := os.ReadFile(backgroundBundle)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	text := string(data)
	for _, patch := range patches {
		text = strings.ReplaceAll(text, patch.Old, patch.New)
	}
	_, err = fileutil.WriteTextIfChanged(backgroundBundle, text)
	return err
}

func ChromeStoreCRXDownloadURL(updateURL, id string) (string, error) {
	parsed, err := url.Parse(updateURL)
	if err != nil {
		return "", fmt.Errorf("parse Chrome Store update URL for %s: %w", id, err)
	}
	values, err := query.Values(chromeStoreCRXQuery{
		Response:     "redirect",
		ProdVersion:  "140.0.0.0",
		AcceptFormat: "crx2,crx3",
		X:            "id=" + id + "&uc",
	})
	if err != nil {
		return "", fmt.Errorf("encode Chrome Store CRX query for %s: %w", id, err)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func ChromeStoreVersionFromCRXURL(id, crxURL string) (string, error) {
	parsed, err := url.Parse(crxURL)
	if err != nil {
		return "", fmt.Errorf("parse Chrome Store CRX URL for %s: %w", id, err)
	}
	file := filepath.Base(parsed.Path)
	prefix := strings.ToUpper(id) + "_"
	if !strings.HasPrefix(file, prefix) || !strings.HasSuffix(file, ".crx") {
		return "", fmt.Errorf("parse Chrome Store CRX version for %s from %s", id, crxURL)
	}
	version := strings.TrimSuffix(strings.TrimPrefix(file, prefix), ".crx")
	return strings.ReplaceAll(version, "_", "."), nil
}

func UnpackedExtensionID(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	id := make([]byte, 32)
	for i, value := range sum[:16] {
		id[i*2] = 'a' + value>>4
		id[i*2+1] = 'a' + value&0x0f
	}
	return string(id)
}

func verifyCRXID(path, want string) (err error) {
	defer func() {
		if recover() != nil {
			err = nil
		}
	}()
	got, err := crx3.ID(path)
	if err != nil {
		return nil
	}
	if got != want {
		return fmt.Errorf("CRX %s has extension ID %s, want %s", path, got, want)
	}
	return nil
}

func writeExternalJSON(path string, value map[string]string) error {
	_, err := fileutil.WriteJSONIfChanged(path, value, 0o644)
	return err
}
