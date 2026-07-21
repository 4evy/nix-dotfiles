package chromiumbrowser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	"github.com/4evy/dotfiles/internal/common/process"
	"github.com/4evy/dotfiles/internal/common/userdirs"
	"gopkg.in/ini.v1"
)

const (
	linuxAppDirName               = "app"
	linuxApplicationsDir          = "applications"
	linuxApplicationIconsDir      = "icons/hicolor/256x256/apps"
	linuxQtShimFilename           = "libqt5_shim.so"
	linuxClassFlagPrefix          = "--class="
	desktopEntryExecKey           = "Exec"
	desktopEntrySection           = "Desktop Entry"
	desktopEntryStartupNotifyKey  = "StartupNotify"
	desktopEntryStartupWMClassKey = "StartupWMClass"
	desktopDatabaseCommand        = "update-desktop-database"
)

func (browser Browser) installLinux(options *InstallOptions) error {
	appDir := filepath.Join(options.Root, linuxAppDirName)
	if options.AppDir != "" {
		appDir = options.AppDir
	}
	dataHome := userdirs.DataHome(homeDir())

	for _, dir := range []string{
		filepath.Join(dataHome, linuxApplicationsDir),
		filepath.Join(dataHome, linuxApplicationIconsDir),
	} {
		if err := os.MkdirAll(dir, fileutil.DefaultDirPerm); err != nil {
			return err
		}
	}
	if err := browser.prepareInstall(options, appDir); err != nil {
		return err
	}
	if err := os.Remove(
		filepath.Join(appDir, linuxQtShimFilename),
	); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
		return err
	}

	linuxWrapperFlags := slices.Clone(browser.Config.Linux.WrapperFlags)
	if browser.Config.Linux.DesktopID != "" {
		linuxWrapperFlags = append(
			linuxWrapperFlags,
			linuxClassFlagPrefix+browser.Config.Linux.DesktopID,
		)
	}
	options.extraWrapperFlags = slices.Insert(
		options.extraWrapperFlags,
		0,
		linuxWrapperFlags...,
	)
	if err := browser.configureApp(
		options,
		filepath.Join(appDir, browser.Config.Linux.LauncherName),
	); err != nil {
		return err
	}

	desktopData, err := os.ReadFile(filepath.Join(appDir, browser.Config.Linux.DesktopName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		executable := filepath.Join(options.BinDir, browser.Config.ExecutableName)
		text, err := LinuxDesktopEntry(
			string(desktopData),
			executable,
			browser.Config.Linux.DesktopExec,
			browser.Config.Linux.DesktopID,
		)
		if err != nil {
			return err
		}
		if _, err := fileutil.WriteTextIfChanged(
			filepath.Join(
				dataHome,
				linuxApplicationsDir,
				browser.Config.ExecutableName+desktopEntryFileSuffix,
			),
			text,
		); err != nil {
			return err
		}
		if err := updateDesktopDatabase(filepath.Join(dataHome, linuxApplicationsDir)); err != nil {
			return err
		}
	}

	iconSource := filepath.Join(appDir, browser.Config.Linux.IconSource)
	if _, err := os.Stat(iconSource); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	} else if err == nil {
		if err := fileutil.CopyPath(
			iconSource,
			filepath.Join(dataHome, linuxApplicationIconsDir, browser.Config.Linux.IconName),
		); err != nil {
			return err
		}
	}
	return nil
}

func LinuxDesktopEntry(text, executable, sourceExec, startupWMClass string) (string, error) {
	cfg, err := ini.LoadSources(ini.LoadOptions{
		Insensitive:         false,
		InsensitiveSections: false,
		InsensitiveKeys:     false,
		IgnoreInlineComment: true,
	}, []byte(text))
	if err != nil {
		return "", fmt.Errorf("parse desktop entry: %w", err)
	}
	for _, section := range cfg.Sections() {
		key, err := section.GetKey(desktopEntryExecKey)
		if err != nil {
			continue
		}
		command, args, _ := strings.Cut(key.String(), " ")
		if command == sourceExec {
			replacement := executable
			if args != "" {
				replacement += " " + args
			}
			key.SetValue(replacement)
		}
	}
	desktop := cfg.Section(desktopEntrySection)
	desktop.Key(desktopEntryStartupNotifyKey).SetValue("false")
	if startupWMClass != "" {
		desktop.Key(desktopEntryStartupWMClassKey).SetValue(startupWMClass)
	}
	var output strings.Builder
	if _, err := cfg.WriteTo(&output); err != nil {
		return "", fmt.Errorf("render desktop entry: %w", err)
	}
	return output.String(), nil
}

func updateDesktopDatabase(applicationsDir string) error {
	if _, err := exec.LookPath(desktopDatabaseCommand); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil
		}
		return err
	}
	return process.RunInWithEnvAndStdin(
		"",
		[]string{desktopDatabaseCommand, applicationsDir},
		nil,
		nil,
	)
}
