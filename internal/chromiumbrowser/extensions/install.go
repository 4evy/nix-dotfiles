package extensions

import "github.com/4evy/dotfiles/internal/common/chromiumext"

type Options = chromiumext.Options

func Install(options Options) (chromiumext.Result, error) {
	catalog, err := LoadCatalog()
	if err != nil {
		return chromiumext.Result{}, err
	}

	options.Catalog = catalog
	return chromiumext.Install(options)
}
