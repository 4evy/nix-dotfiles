package chromiumbrowser

import (
	"context"
	"path/filepath"

	"github.com/4evy/dotfiles/internal/chromiumbrowser/extensions"
	"github.com/4evy/dotfiles/internal/common/archiveutil"
	"github.com/4evy/dotfiles/internal/common/httpx"
)

func (browser Browser) installExtensions(options *InstallOptions) error {
	result, err := extensions.Install(extensions.Options{
		Root:          options.Root,
		ExternalDirs:  browser.ExternalDirs(options.Mode),
		Download:      downloadFile,
		Resolve:       resolveDownloadURL,
		BundlePatches: options.BundlePatches,
		Unzip: func(zipPath, dst string) error {
			return archiveutil.ExtractZipFile(context.Background(), zipPath, dst)
		},
	})
	if err != nil {
		return err
	}
	for _, path := range result.LoadExtensionPaths {
		options.extraWrapperFlags = append(options.extraWrapperFlags, "--load-extension="+path)
	}
	aliases, err := unpackedExtensionIDAliases(options.Root)
	if err != nil {
		return err
	}
	options.extensionIDAliases = aliases
	return nil
}

func unpackedExtensionIDAliases(root string) (map[string]string, error) {
	catalog, err := extensions.LoadCatalog()
	if err != nil {
		return nil, err
	}
	aliases := map[string]string{}
	for _, extension := range catalog.ZIP {
		if !extension.LoadUnpacked {
			continue
		}
		path := filepath.Join(root, "extensions/unpacked", extension.ID)
		aliases[extension.ID] = extensions.UnpackedExtensionID(path)
	}
	return aliases, nil
}

func downloadFile(path, url string) error {
	return (&httpx.Client{}).DownloadFile(url, path)
}

func resolveDownloadURL(rawURL string) (string, error) {
	return (&httpx.Client{}).ResolveURL(rawURL)
}
