package chromiumext

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/4evy/dotfiles/internal/common/fileutil"
)

func TestVerifyFileSHA256(t *testing.T) {
	data := []byte("verified extension")
	digest := sha256.Sum256(data)
	checksum := sha256SRIprefix + base64.StdEncoding.EncodeToString(digest[:])
	path := filepath.Join(t.TempDir(), "extension.crx")
	if err := os.WriteFile(path, data, fileutil.PrivateFilePerm); err != nil {
		t.Fatal(err)
	}

	if err := VerifyFileSHA256(path, checksum); err != nil {
		t.Fatalf("verify matching checksum: %v", err)
	}
	if err := VerifyFileSHA256(
		path,
		"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	); err == nil {
		t.Fatal("expected mismatched checksum to fail")
	}
	hexChecksum := "sha256:" + hex.EncodeToString(digest[:])
	if err := VerifyFileSHA256(path, hexChecksum); err != nil {
		t.Fatalf("verify GitHub digest: %v", err)
	}
	wantSRI := "sha256-" + base64.StdEncoding.EncodeToString(digest[:])
	if got, err := NormalizeSHA256(hexChecksum); err != nil || got != wantSRI {
		t.Fatalf("NormalizeSHA256(%q) = %q, %v; want %q", hexChecksum, got, err, wantSRI)
	}
}

func TestVerifyCRXIDRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "extension.crx")
	if err := os.WriteFile(path, []byte("not a CRX"), fileutil.PrivateFilePerm); err != nil {
		t.Fatal(err)
	}
	if err := verifyCRXID(path, "aeblfdkhhhdcdjpifhhbdiojplfjncoa"); err == nil {
		t.Fatal("expected malformed CRX to fail verification")
	}
}

func TestInstallRequiresDownloader(t *testing.T) {
	if _, err := Install(Options{}); err == nil {
		t.Fatal("expected missing downloader to fail")
	}
}

func TestInstallLoadsVerifiedZIPAsUnpackedExtension(t *testing.T) {
	root := t.TempDir()
	const extensionID = "cjpalhdlnbpafiamejdnhcphjbkeiagm"
	result, err := Install(Options{
		Root: root,
		Catalog: Catalog{ZIP: []GitHubReleaseExtension{{
			ID:            extensionID,
			Name:          "uBlock Origin",
			Repository:    "gorhill/uBlock",
			AssetTemplate: "uBlock0_{tag}.chromium.zip",
			ArchiveRoot:   "uBlock0.chromium",
			LoadUnpacked:  true,
		}}},
		Download: func(target, url string) error {
			if url != "https://github.com/gorhill/uBlock/releases/download/1.72.2/uBlock0_1.72.2.chromium.zip" {
				t.Fatalf("download URL = %q", url)
			}
			return writeTestZIP(target, map[string]string{
				"uBlock0.chromium/manifest.json":   `{"name":"uBlock Origin","version":"1.72.2"}`,
				"uBlock0.chromium/background.html": "uBlock Origin",
			})
		},
		ResolveLatestRelease: func(repository, assetTemplate string) (ReleaseArtifact, error) {
			if repository != "gorhill/uBlock" || assetTemplate != "uBlock0_{tag}.chromium.zip" {
				t.Fatalf("release request = %q, %q", repository, assetTemplate)
			}
			return ReleaseArtifact{
				Version: "1.72.2",
				URL:     "https://github.com/gorhill/uBlock/releases/download/1.72.2/uBlock0_1.72.2.chromium.zip",
				SHA256:  "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			}, nil
		},
		Verify: func(_ string, checksum string) error {
			if checksum != "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
				t.Fatalf("checksum = %q", checksum)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.LoadExtensionPaths) != 1 {
		t.Fatalf("load-extension paths = %q", result.LoadExtensionPaths)
	}
	extensionPath := filepath.Join(root, "extensions/unpacked", extensionID)
	if result.LoadExtensionPaths[0] != extensionPath {
		t.Fatalf("load-extension path = %q, want %q", result.LoadExtensionPaths[0], extensionPath)
	}
	if got := result.ExtensionIDAliases[extensionID]; got != UnpackedExtensionID(extensionPath) {
		t.Fatalf("unpacked extension ID = %q, want %q", got, UnpackedExtensionID(extensionPath))
	}
	data, err := os.ReadFile(filepath.Join(extensionPath, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.72.2" {
		t.Fatalf("installed manifest version = %q", manifest.Version)
	}
}

func writeTestZIP(target string, files map[string]string) (err error) {
	if err := os.MkdirAll(filepath.Dir(target), fileutil.DefaultDirPerm); err != nil {
		return err
	}
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(file)
	for name, content := range files {
		entry, err := archive.Create(name)
		if err != nil {
			_ = archive.Close()
			_ = file.Close()
			return err
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			_ = archive.Close()
			_ = file.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
