package archiveutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
)

func ExtractZipFile(ctx context.Context, zipPath, dst string) (err error) {
	file, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	return extractArchive(ctx, archives.Zip{}, file, dst)
}

func ExtractTarGz(ctx context.Context, source io.Reader, dst string) error {
	return extractArchive(ctx, archives.CompressedArchive{
		Compression: archives.Gz{},
		Extraction:  archives.Tar{},
	}, source, dst)
}

func ExtractTarXz(ctx context.Context, source io.Reader, dst string) error {
	return extractArchive(ctx, archives.CompressedArchive{
		Compression: archives.Xz{},
		Extraction:  archives.Tar{},
	}, source, dst)
}

func extractArchive(
	ctx context.Context,
	extractor archives.Extractor,
	source io.Reader,
	dst string,
) (err error) {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	return extractor.Extract(ctx, source, func(ctx context.Context, file archives.FileInfo) error {
		return extractArchiveFile(root, file)
	})
}

func extractArchiveFile(root *os.Root, file archives.FileInfo) (err error) {
	if !file.Mode().IsRegular() && !file.IsDir() {
		return nil
	}
	if file.IsDir() {
		return extractEntry(root, file.NameInArchive, file.Mode(), nil)
	}
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	return extractEntry(root, file.NameInArchive, file.Mode(), source)
}

func extractEntry(root *os.Root, name string, mode fs.FileMode, source io.Reader) error {
	target, err := safeLocalPath(name)
	if err != nil {
		return err
	}
	if target == "" {
		return nil
	}
	if mode.IsDir() {
		return root.MkdirAll(target, permOrDefault(mode, 0o755))
	}
	if !mode.IsRegular() {
		return nil
	}
	if err := root.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	destination, err := root.OpenFile(
		target,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		permOrDefault(mode, 0o644),
	)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func safeLocalPath(name string) (string, error) {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == "." {
		return "", nil
	}
	if path.IsAbs(clean) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	candidate, err := filepath.Localize(clean)
	if err != nil || !filepath.IsLocal(candidate) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	return candidate, nil
}

func permOrDefault(mode fs.FileMode, fallback fs.FileMode) fs.FileMode {
	if perm := mode.Perm(); perm != 0 {
		return perm
	}
	return fallback
}
