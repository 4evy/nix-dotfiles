package chromiumbrowser

import (
	"context"
	"strings"

	"github.com/4evy/dotfiles/internal/chromiumbrowser/extensions"
	"github.com/4evy/dotfiles/internal/common/chromiumext"
	"github.com/4evy/dotfiles/internal/common/githubx"
	"github.com/4evy/dotfiles/internal/common/httpx"
)

const (
	loadExtensionFlagPrefix = "--load-extension="
	releaseTagPlaceholder   = "{tag}"
	releaseTagPrefix        = "v"
)

func (browser Browser) installExtensions(options *InstallOptions) error {
	result, err := extensions.Install(extensions.Options{
		Root:                 options.Root,
		ExternalDirs:         browser.Config.ExternalExtensionDirs(options.Mode),
		Download:             downloadFile,
		Resolve:              resolveDownloadURL,
		ResolveLatestRelease: resolveLatestGitHubRelease,
		ExcludedIDs:          browser.extensionInstallExclusions(),
	})
	if err != nil {
		return err
	}
	for _, path := range result.LoadExtensionPaths {
		options.extraWrapperFlags = append(options.extraWrapperFlags, loadExtensionFlagPrefix+path)
	}
	options.extensionIDAliases = result.ExtensionIDAliases
	return nil
}

func resolveLatestGitHubRelease(
	repository,
	assetTemplate string,
) (chromiumext.ReleaseArtifact, error) {
	tag, err := githubx.LatestReleaseTag(context.Background(), repository)
	if err != nil {
		return chromiumext.ReleaseArtifact{}, err
	}
	assetName := strings.ReplaceAll(assetTemplate, releaseTagPlaceholder, tag)
	asset, err := githubx.ReleaseAsset(context.Background(), repository, tag, assetName)
	if err != nil {
		return chromiumext.ReleaseArtifact{}, err
	}
	checksum, err := chromiumext.NormalizeSHA256(asset.Digest)
	if err != nil {
		return chromiumext.ReleaseArtifact{}, err
	}
	return chromiumext.ReleaseArtifact{
		Version: strings.TrimPrefix(tag, releaseTagPrefix),
		URL:     asset.DownloadURL,
		SHA256:  checksum,
	}, nil
}

func (browser Browser) extensionInstallExclusions() map[string]bool {
	excludedIDs := map[string]bool{}
	for sourceID, installedID := range browser.Config.ExtensionIDAliases {
		if sourceID != installedID {
			excludedIDs[sourceID] = true
		}
	}
	return excludedIDs
}

func downloadFile(path, url string) error {
	return (&httpx.Client{}).DownloadFile(url, path)
}

func resolveDownloadURL(rawURL string) (string, error) {
	return (&httpx.Client{}).ResolveURL(rawURL)
}
