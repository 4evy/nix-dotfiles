package extensions

import (
	_ "embed"
	"fmt"

	"github.com/4evy/dotfiles/internal/common/chromiumext"
	"github.com/pelletier/go-toml/v2"
)

//go:embed patches.toml
var patchData []byte

type patchManifest struct {
	Patches []namedBundlePatch `toml:"patch"`
}

type namedBundlePatch struct {
	Name string `toml:"name"`
	Old  string `toml:"old"`
	New  string `toml:"new"`
}

var bundledPatches = mustLoadBundlePatches(patchData)

var (
	SuppressInstallOptionsPagePatch  = bundledPatches["suppress_install_options_page"]
	DisableOpenOptionsPageCallsPatch = bundledPatches["disable_open_options_page_calls"]
)

func mustLoadBundlePatches(data []byte) map[string]chromiumext.BundlePatch {
	patches, err := loadBundlePatches(data)
	if err != nil {
		panic(err)
	}
	return patches
}

func loadBundlePatches(data []byte) (map[string]chromiumext.BundlePatch, error) {
	var manifest patchManifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse embedded Chromium extension patches: %w", err)
	}
	patches := make(map[string]chromiumext.BundlePatch, len(manifest.Patches))
	for _, patch := range manifest.Patches {
		if patch.Name == "" || patch.Old == "" {
			return nil, fmt.Errorf("embedded Chromium extension patch %q is incomplete", patch.Name)
		}
		if _, exists := patches[patch.Name]; exists {
			return nil, fmt.Errorf("embedded Chromium extension patch %q is duplicated", patch.Name)
		}
		patches[patch.Name] = chromiumext.BundlePatch{Old: patch.Old, New: patch.New}
	}
	for _, name := range []string{
		"suppress_install_options_page",
		"disable_open_options_page_calls",
	} {
		if _, ok := patches[name]; !ok {
			return nil, fmt.Errorf("embedded Chromium extension patches are missing %s", name)
		}
	}
	return patches, nil
}
