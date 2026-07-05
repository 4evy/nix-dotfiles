package extensions

import (
	_ "embed"
	"fmt"

	"github.com/4evy/dotfiles/internal/common/chromiumext"
	"github.com/pelletier/go-toml/v2"
)

//go:embed extensions.toml
var catalogData []byte

type (
	Catalog              = chromiumext.Catalog
	ChromeStoreExtension = chromiumext.ChromeStoreExtension
	UpdateURLExtension   = chromiumext.UpdateURLExtension
	DownloadedExtension  = chromiumext.DownloadedExtension
)

func LoadCatalog() (Catalog, error) {
	var catalog Catalog
	if err := toml.Unmarshal(catalogData, &catalog); err != nil {
		return catalog, fmt.Errorf("parse embedded Chromium extension catalog: %w", err)
	}
	if err := chromiumext.ValidateCatalog(catalog, "chromium"); err != nil {
		return catalog, err
	}
	return catalog, nil
}

func validExtensionID(id string) bool {
	return chromiumext.ValidExtensionID(id)
}

func validExternalVersion(version string) bool {
	return chromiumext.ValidExternalVersion(version)
}
