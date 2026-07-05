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
	"github.com/4evy/dotfiles/internal/common/userdirs"
)

func (browser Browser) installLinux(options *InstallOptions) error {
	appDir := browser.defaultAppDir(options.Root)
	if options.AppDir != "" {
		appDir = options.AppDir
	}
	dataHome := userdirs.DataHome(homeDir())

	for _, dir := range []string{
		options.Root,
		options.BinDir,
		filepath.Join(dataHome, "applications"),
		filepath.Join(dataHome, "icons/hicolor/256x256/apps"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if stat, err := os.Stat(appDir); err != nil {
		return fmt.Errorf("find %s app directory %s: %w", browser.Name, appDir, err)
	} else if !stat.IsDir() {
		return fmt.Errorf("%s app path is not a directory: %s", browser.Name, appDir)
	}
	if err := os.Remove(
		filepath.Join(appDir, "libqt5_shim.so"),
	); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := browser.installExtensions(options); err != nil {
		return err
	}
	if err := browser.applyInstallSettings(options); err != nil {
		return err
	}
	linuxWrapperFlags := slices.Clone(browser.LinuxWrapperFlags)
	if browser.LinuxDesktopID != "" {
		linuxWrapperFlags = append(linuxWrapperFlags, "--class="+browser.LinuxDesktopID)
	}
	options.extraWrapperFlags = slices.Insert(
		options.extraWrapperFlags,
		0,
		linuxWrapperFlags...,
	)
	if err := writeWrapper(
		filepath.Join(options.BinDir, browser.ExecutableName),
		filepath.Join(appDir, browser.LinuxLauncherName),
		options,
	); err != nil {
		return err
	}
	if browser.AliasName != "" {
		if err := replaceSymlink(
			browser.ExecutableName,
			filepath.Join(options.BinDir, browser.AliasName),
		); err != nil {
			return err
		}
	}

	desktopData, err := os.ReadFile(filepath.Join(appDir, browser.LinuxDesktopName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		executable := filepath.Join(options.BinDir, browser.ExecutableName)
		text := LinuxDesktopEntry(
			string(desktopData),
			executable,
			browser.LinuxDesktopExec,
			browser.LinuxDesktopID,
		)
		if _, err := fileutil.WriteTextIfChanged(
			filepath.Join(dataHome, "applications", browser.ExecutableName+".desktop"),
			text,
		); err != nil {
			return err
		}
		if err := updateDesktopDatabase(filepath.Join(dataHome, "applications")); err != nil {
			return err
		}
	}

	iconSource := filepath.Join(appDir, browser.LinuxIconSource)
	if _, err := os.Stat(iconSource); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	} else if err == nil {
		if err := fileutil.CopyPath(
			iconSource,
			filepath.Join(dataHome, "icons/hicolor/256x256/apps", browser.LinuxIconName),
		); err != nil {
			return err
		}
	}
	return nil
}

func LinuxDesktopEntry(text, executable, sourceExec, startupWMClass string) string {
	text = strings.ReplaceAll(text, "Exec="+sourceExec+" %U", "Exec="+executable+" %U")
	text = strings.ReplaceAll(
		text,
		"Exec="+sourceExec+" --incognito",
		"Exec="+executable+" --incognito",
	)
	text = strings.ReplaceAll(text, "Exec="+sourceExec+"\n", "Exec="+executable+"\n")
	text = setDesktopEntryKey(text, "StartupNotify", "false")
	if startupWMClass == "" {
		return text
	}
	return setDesktopEntryKey(text, "StartupWMClass", startupWMClass)
}

func setDesktopEntryKey(text, key, value string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "[Desktop Entry]" {
		return text
	}

	keyPrefix := key + "="
	insertAt := len(lines)
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, keyPrefix) {
			lines[i] = keyPrefix + value
			return strings.Join(lines, "\n")
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			insertAt = i
			break
		}
	}

	lines = slices.Insert(lines, insertAt, keyPrefix+value)
	return strings.Join(lines, "\n")
}

func updateDesktopDatabase(applicationsDir string) error {
	if _, err := exec.LookPath("update-desktop-database"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil
		}
		return err
	}
	return run("update-desktop-database", applicationsDir)
}

func run(name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
