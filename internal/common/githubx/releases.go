package githubx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/4evy/dotfiles/internal/common/httpx"
	"github.com/google/go-github/v89/github"
)

const (
	githubRequestTimeout = 30 * time.Second
	releaseAssetsPerPage = 100
)

func LatestReleaseTag(ctx context.Context, repository string) (string, error) {
	owner, name, err := SplitRepository(repository)
	if err != nil {
		return "", err
	}
	client, err := github.NewClient(
		github.WithHTTPClient(httpx.RetryableClient(githubRequestTimeout)),
	)
	if err != nil {
		return "", err
	}
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, name)
	if err != nil {
		return "", fmt.Errorf("get latest release for %s: %w", repository, err)
	}
	tag := release.GetTagName()
	if tag == "" {
		return "", fmt.Errorf("latest release for %s has no tag name", repository)
	}
	return tag, nil
}

type ReleaseAssetInfo struct {
	TagName     string
	Name        string
	DownloadURL string
	Digest      string
}

func LatestReleaseAsset(ctx context.Context, repository, assetName string) (ReleaseAssetInfo, error) {
	tag, err := LatestReleaseTag(ctx, repository)
	if err != nil {
		return ReleaseAssetInfo{}, err
	}
	return ReleaseAsset(ctx, repository, tag, assetName)
}

func ReleaseAsset(ctx context.Context, repository, tag, assetName string) (ReleaseAssetInfo, error) {
	owner, name, err := SplitRepository(repository)
	if err != nil {
		return ReleaseAssetInfo{}, err
	}
	client, err := github.NewClient(
		github.WithHTTPClient(httpx.RetryableClient(githubRequestTimeout)),
	)
	if err != nil {
		return ReleaseAssetInfo{}, err
	}
	release, _, err := client.Repositories.GetReleaseByTag(ctx, owner, name, tag)
	if err != nil {
		return ReleaseAssetInfo{}, fmt.Errorf("get release %s for %s: %w", tag, repository, err)
	}
	releaseID := release.GetID()
	if releaseID == 0 {
		return ReleaseAssetInfo{}, fmt.Errorf("release %s for %s has no release ID", tag, repository)
	}

	opt := &github.ListOptions{PerPage: releaseAssetsPerPage}
	for {
		assets, resp, err := client.Repositories.ListReleaseAssets(ctx, owner, name, releaseID, opt)
		if err != nil {
			return ReleaseAssetInfo{}, fmt.Errorf("list assets for %s release %s: %w", repository, tag, err)
		}
		for _, asset := range assets {
			if asset.GetName() != assetName {
				continue
			}
			url := asset.GetBrowserDownloadURL()
			if url == "" {
				return ReleaseAssetInfo{}, fmt.Errorf("asset %s in %s release %s has no download URL", assetName, repository, tag)
			}
			return ReleaseAssetInfo{
				TagName:     tag,
				Name:        assetName,
				DownloadURL: url,
				Digest:      asset.GetDigest(),
			}, nil
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return ReleaseAssetInfo{}, fmt.Errorf("asset %s not found in %s release %s", assetName, repository, tag)
}

func SplitRepository(repository string) (string, string, error) {
	owner, name, ok := strings.Cut(repository, "/")
	if ok && owner != "" && name != "" {
		return owner, name, nil
	}
	return "", "", fmt.Errorf("invalid GitHub repository %q", repository)
}
