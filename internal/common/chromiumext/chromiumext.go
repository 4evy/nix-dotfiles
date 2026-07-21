package chromiumext

import (
	"archive/zip"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	crx3 "github.com/mediabuyerbot/go-crx3"
)

const (
	crxExtensionDir               = "extensions/crx"
	zipExtensionArchiveDir        = "extensions/zip"
	unpackedExtensionDir          = "extensions/unpacked"
	crxFileExtension              = ".crx"
	zipFileExtension              = ".zip"
	jsonFileExtension             = ".json"
	manifestFilename              = "manifest.json"
	externalCRXKey                = "external_crx"
	externalVersionKey            = "external_version"
	externalUpdateURLKey          = "external_update_url"
	releaseTagPlaceholder         = "{tag}"
	sha256SRIprefix               = "sha256-"
	sha256HexPrefix               = "sha256:"
	extensionIDLength             = 32
	extensionIDHashBytes          = extensionIDLength / 2
	extensionIDFirstCharacter     = 'a'
	extensionIDLastCharacter      = 'p'
	bitsPerNibble                 = 4
	nibblesPerByte                = 2
	lowNibbleOffset               = 1
	nibbleMask                    = 0x0f
	constantTimeEqual             = 1
	decimalDigitFirst             = '0'
	decimalDigitLast              = '9'
	chromeStoreResponseQueryKey   = "response"
	chromeStoreProductQueryKey    = "prodversion"
	chromeStoreFormatQueryKey     = "acceptformat"
	chromeStoreExtensionQueryKey  = "x"
	chromeStoreResponse           = "redirect"
	chromeStoreProductVersion     = "150.0.0.0"
	chromeStoreAcceptedFormats    = "crx2,crx3"
	chromeStoreInstallQuerySuffix = "&uc"
)

type Catalog struct {
	ChromeStoreUpdateURL string                   `toml:"chrome_store_update_url"`
	ChromeStore          []ChromeStoreExtension   `toml:"chrome_store_extensions"`
	UpdateURL            []UpdateURLExtension     `toml:"update_url_extensions"`
	CRX                  []DownloadedExtension    `toml:"crx_extensions"`
	ZIP                  []GitHubReleaseExtension `toml:"zip_extensions"`
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
	ID      string `toml:"id"`
	Name    string `toml:"name"`
	Version string `toml:"version"`
	URL     string `toml:"url"`
	SHA256  string `toml:"sha256"`
}

type GitHubReleaseExtension struct {
	ID            string `toml:"id"`
	Name          string `toml:"name"`
	Repository    string `toml:"repository"`
	AssetTemplate string `toml:"asset_template"`
	ArchiveRoot   string `toml:"archive_root"`
	LoadUnpacked  bool   `toml:"load_unpacked"`
}

type ReleaseArtifact struct {
	Version string
	URL     string
	SHA256  string
}

type Options struct {
	Root                 string
	ExternalDirs         []string
	Catalog              Catalog
	Download             func(path, url string) error
	Resolve              func(url string) (string, error)
	Verify               func(path, checksum string) error
	VerifyCRX            func(path, extensionID string) error
	ResolveLatestRelease func(repository, assetTemplate string) (ReleaseArtifact, error)
	ExcludedIDs          map[string]bool
}

type Result struct {
	LoadExtensionPaths []string
	ExtensionIDAliases map[string]string
}

type installedCRXExtension struct {
	DownloadedExtension
	Path string
}

func Install(options Options) (Result, error) {
	if options.Download == nil {
		return Result{}, errors.New("chromium extension download function is required")
	}
	verify := options.Verify
	if verify == nil {
		verify = VerifyFileSHA256
	}
	verifyCRX := options.VerifyCRX
	if verifyCRX == nil {
		verifyCRX = verifyCRXID
	}
	crxDir := filepath.Join(options.Root, crxExtensionDir)
	if err := os.MkdirAll(crxDir, fileutil.DefaultDirPerm); err != nil {
		return Result{}, err
	}

	crxExtensions := make(
		[]installedCRXExtension,
		0,
		len(options.Catalog.CRX)+len(options.Catalog.ChromeStore),
	)
	for _, extension := range options.Catalog.ChromeStore {
		if options.ExcludedIDs[extension.ID] {
			continue
		}
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
		crxPath := filepath.Join(crxDir, extension.ID+crxFileExtension)
		if err := options.Download(crxPath, crxURL); err != nil {
			return Result{}, err
		}
		if err := verifyCRX(crxPath, extension.ID); err != nil {
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
		if options.ExcludedIDs[extension.ID] {
			continue
		}
		crxPath := filepath.Join(crxDir, extension.ID+crxFileExtension)
		if err := options.Download(crxPath, extension.URL); err != nil {
			return Result{}, err
		}
		if err := verify(crxPath, extension.SHA256); err != nil {
			return Result{}, fmt.Errorf("verify %s: %w", extension.Name, err)
		}
		if err := verifyCRX(crxPath, extension.ID); err != nil {
			return Result{}, err
		}
		crxExtensions = append(crxExtensions, installedCRXExtension{
			DownloadedExtension: extension,
			Path:                crxPath,
		})
	}

	result := Result{ExtensionIDAliases: map[string]string{}}
	for _, extension := range options.Catalog.ZIP {
		if options.ExcludedIDs[extension.ID] {
			continue
		}
		extensionPath, err := installUnpackedExtension(options, verify, extension)
		if err != nil {
			return Result{}, err
		}
		if extension.LoadUnpacked {
			result.LoadExtensionPaths = append(result.LoadExtensionPaths, extensionPath)
			result.ExtensionIDAliases[extension.ID] = UnpackedExtensionID(extensionPath)
		}
	}

	for _, externalDir := range options.ExternalDirs {
		if err := os.MkdirAll(externalDir, fileutil.DefaultDirPerm); err != nil {
			return Result{}, err
		}
		for _, extension := range crxExtensions {
			if err := writeExternalJSON(
				filepath.Join(externalDir, extension.ID+jsonFileExtension),
				map[string]string{
					externalCRXKey:     extension.Path,
					externalVersionKey: extension.Version,
				},
			); err != nil {
				return Result{}, err
			}
		}
		for _, extension := range options.Catalog.UpdateURL {
			if options.ExcludedIDs[extension.ID] {
				continue
			}
			if err := writeExternalJSON(
				filepath.Join(externalDir, extension.ID+jsonFileExtension),
				map[string]string{
					externalUpdateURLKey: extension.UpdateURL,
				},
			); err != nil {
				return Result{}, err
			}
		}
	}

	return result, nil
}

func installUnpackedExtension(
	options Options,
	verify func(path, checksum string) error,
	extension GitHubReleaseExtension,
) (_ string, err error) {
	if options.ResolveLatestRelease == nil {
		return "", errors.New("latest GitHub release resolver is required")
	}
	artifact, err := options.ResolveLatestRelease(extension.Repository, extension.AssetTemplate)
	if err != nil {
		return "", fmt.Errorf("resolve latest %s release: %w", extension.Name, err)
	}
	if !ValidExternalVersion(artifact.Version) ||
		!ValidExternalUpdateURL(artifact.URL) ||
		!ValidSHA256(artifact.SHA256) {
		return "", fmt.Errorf("latest %s release metadata is incomplete", extension.Name)
	}
	archiveDir := filepath.Join(options.Root, zipExtensionArchiveDir)
	unpackedDir := filepath.Join(options.Root, unpackedExtensionDir)
	for _, dir := range []string{archiveDir, unpackedDir} {
		if err := os.MkdirAll(dir, fileutil.DefaultDirPerm); err != nil {
			return "", err
		}
	}
	archivePath := filepath.Join(
		archiveDir,
		extension.ID+"-"+artifact.Version+zipFileExtension,
	)
	if err := options.Download(archivePath, artifact.URL); err != nil {
		return "", err
	}
	if err := verify(archivePath, artifact.SHA256); err != nil {
		return "", fmt.Errorf("verify %s: %w", extension.Name, err)
	}

	temporaryDir, err := os.MkdirTemp(unpackedDir, "."+extension.ID+"-")
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temporaryDir)) }()
	if err := ExtractZipFile(archivePath, temporaryDir); err != nil {
		return "", fmt.Errorf("extract %s: %w", extension.Name, err)
	}
	sourceDir := filepath.Join(temporaryDir, filepath.FromSlash(extension.ArchiveRoot))
	if err := validateUnpackedManifest(sourceDir, extension.Name, artifact.Version); err != nil {
		return "", err
	}
	extensionDir := filepath.Join(unpackedDir, extension.ID)
	if err := os.RemoveAll(extensionDir); err != nil {
		return "", err
	}
	if err := os.Rename(sourceDir, extensionDir); err != nil {
		return "", err
	}
	return extensionDir, nil
}

func validateUnpackedManifest(extensionDir, name, version string) error {
	data, err := os.ReadFile(filepath.Join(extensionDir, manifestFilename))
	if err != nil {
		return fmt.Errorf("read %s manifest: %w", name, err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse %s manifest: %w", name, err)
	}
	if manifest.Version != version {
		return fmt.Errorf(
			"%s manifest version is %q, want %q",
			name,
			manifest.Version,
			version,
		)
	}
	return nil
}

func ExtractZipFile(zipPath, dst string) (err error) {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, archive.Close()) }()
	root, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, entry := range archive.File {
		if err := extractZipEntry(root, entry); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(root *os.Root, entry *zip.File) (err error) {
	clean := path.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
	if clean == "." {
		return nil
	}
	target, err := filepath.Localize(clean)
	if err != nil || !filepath.IsLocal(target) {
		return fmt.Errorf("archive entry escapes destination: %s", entry.Name)
	}
	mode := entry.Mode()
	if mode.IsDir() {
		return root.MkdirAll(target, permOrDefault(mode, fileutil.DefaultDirPerm))
	}
	if !mode.IsRegular() {
		return nil
	}
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	if err := root.MkdirAll(filepath.Dir(target), fileutil.DefaultDirPerm); err != nil {
		return err
	}
	destination, err := root.OpenFile(
		target,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		permOrDefault(mode, fileutil.DefaultFilePerm),
	)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	return errors.Join(copyErr, closeErr)
}

func permOrDefault(mode fs.FileMode, fallback fs.FileMode) fs.FileMode {
	if permission := mode.Perm(); permission != 0 {
		return permission
	}
	return fallback
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
	for _, extension := range catalog.CRX {
		missingID := extension.ID == ""
		missingVersion := extension.Version == ""
		missingURL := extension.URL == ""
		missingChecksum := extension.SHA256 == ""
		if missingID || missingVersion || missingURL || missingChecksum ||
			!ValidExtensionID(extension.ID) ||
			!ValidExternalVersion(extension.Version) ||
			!ValidExternalUpdateURL(extension.URL) ||
			!ValidSHA256(extension.SHA256) {
			return fmt.Errorf(
				"%s extension catalog contains an incomplete downloaded extension entry",
				browser,
			)
		}
	}
	for _, extension := range catalog.ZIP {
		missingID := extension.ID == ""
		missingRepository := extension.Repository == ""
		missingAssetTemplate := extension.AssetTemplate == ""
		invalidArchiveRoot := extension.ArchiveRoot != "" &&
			!filepath.IsLocal(filepath.FromSlash(extension.ArchiveRoot))
		if missingID || missingRepository || missingAssetTemplate ||
			!ValidExtensionID(extension.ID) ||
			!ValidGitHubRepository(extension.Repository) ||
			!strings.Contains(extension.AssetTemplate, releaseTagPlaceholder) || invalidArchiveRoot {
			return fmt.Errorf(
				"%s extension catalog contains an incomplete ZIP extension entry",
				browser,
			)
		}
	}
	return nil
}

func ValidSHA256(checksum string) bool {
	_, err := decodeSHA256(checksum)
	return err == nil
}

func NormalizeSHA256(checksum string) (string, error) {
	digest, err := decodeSHA256(checksum)
	if err != nil {
		return "", err
	}
	return sha256SRIprefix + base64.StdEncoding.EncodeToString(digest), nil
}

func VerifyFileSHA256(path, checksum string) (err error) {
	want, err := decodeSHA256(checksum)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(digest.Sum(nil), want) != constantTimeEqual {
		return fmt.Errorf("SHA-256 checksum mismatch for %s", path)
	}
	return nil
}

func decodeSHA256(checksum string) ([]byte, error) {
	if encoded, ok := strings.CutPrefix(checksum, sha256SRIprefix); ok {
		digest, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil && len(digest) == sha256.Size {
			return digest, nil
		}
	}
	if encoded, ok := strings.CutPrefix(checksum, sha256HexPrefix); ok {
		digest, err := hex.DecodeString(encoded)
		if err == nil && len(digest) == sha256.Size {
			return digest, nil
		}
	}
	return nil, fmt.Errorf("invalid SHA-256 checksum %q", checksum)
}

func ValidExtensionID(id string) bool {
	if len(id) != extensionIDLength {
		return false
	}
	for _, char := range id {
		if char < extensionIDFirstCharacter || char > extensionIDLastCharacter {
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
			if char < decimalDigitFirst || char > decimalDigitLast {
				return false
			}
		}
	}
	return true
}

func ValidGitHubRepository(repository string) bool {
	owner, name, ok := strings.Cut(repository, "/")
	return ok && owner != "" && name != "" && !strings.Contains(name, "/")
}

func UnpackedExtensionID(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	id := make([]byte, extensionIDLength)
	for index, value := range sum[:extensionIDHashBytes] {
		id[index*nibblesPerByte] = extensionIDFirstCharacter + value>>bitsPerNibble
		id[index*nibblesPerByte+lowNibbleOffset] = extensionIDFirstCharacter + value&nibbleMask
	}
	return string(id)
}

func ChromeStoreCRXDownloadURL(updateURL, id string) (string, error) {
	parsed, err := url.Parse(updateURL)
	if err != nil {
		return "", fmt.Errorf("parse Chrome Store update URL for %s: %w", id, err)
	}
	parsed.RawQuery = url.Values{
		chromeStoreResponseQueryKey:  {chromeStoreResponse},
		chromeStoreProductQueryKey:   {chromeStoreProductVersion},
		chromeStoreFormatQueryKey:    {chromeStoreAcceptedFormats},
		chromeStoreExtensionQueryKey: {"id=" + id + chromeStoreInstallQuerySuffix},
	}.Encode()
	return parsed.String(), nil
}

func ChromeStoreVersionFromCRXURL(id, crxURL string) (string, error) {
	parsed, err := url.Parse(crxURL)
	if err != nil {
		return "", fmt.Errorf("parse Chrome Store CRX URL for %s: %w", id, err)
	}
	file := filepath.Base(parsed.Path)
	prefix := strings.ToUpper(id) + "_"
	if !strings.HasPrefix(file, prefix) || !strings.HasSuffix(file, crxFileExtension) {
		return "", fmt.Errorf("parse Chrome Store CRX version for %s from %s", id, crxURL)
	}
	version := strings.TrimSuffix(strings.TrimPrefix(file, prefix), crxFileExtension)
	return strings.ReplaceAll(version, "_", "."), nil
}

func verifyCRXID(path, want string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("read CRX extension ID from %s: %v", path, recovered)
		}
	}()
	got, err := crx3.ID(path)
	if err != nil {
		return fmt.Errorf("read CRX extension ID from %s: %w", path, err)
	}
	if got != want {
		return fmt.Errorf("CRX %s has extension ID %s, want %s", path, got, want)
	}
	return nil
}

func writeExternalJSON(path string, value map[string]string) error {
	_, err := fileutil.WriteJSONIfChanged(path, value, fileutil.DefaultFilePerm)
	return err
}
