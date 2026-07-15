package extensions

import "github.com/4evy/dotfiles/internal/common/chromiumext"

type Options struct {
	Root          string
	ExternalDirs  []string
	Download      func(path, url string) error
	Resolve       func(url string) (string, error)
	Unzip         func(zipPath, dst string) error
	BundlePatches []chromiumext.BundlePatch
}

type Result = chromiumext.Result

func Install(options Options) (Result, error) {
	catalog, err := LoadCatalog()
	if err != nil {
		return Result{}, err
	}

	patches := []chromiumext.BundlePatch{SuppressInstallOptionsPagePatch}
	patches = append(patches, options.BundlePatches...)
	return chromiumext.Install(chromiumext.Options{
		Root:          options.Root,
		ExternalDirs:  options.ExternalDirs,
		Catalog:       catalog,
		Download:      options.Download,
		Resolve:       options.Resolve,
		Unzip:         options.Unzip,
		BundlePatches: patches,
	})
}

func UnpackedExtensionID(path string) string {
	return chromiumext.UnpackedExtensionID(path)
}
