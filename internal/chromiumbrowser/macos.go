package chromiumbrowser

import "path/filepath"

func (browser Browser) installMacOS(options *InstallOptions) error {
	appDir := options.AppDir
	if appDir == "" {
		appDir = expandPathTemplate(browser.Config.MacOS.AppDir)
	}

	if err := browser.prepareInstall(options, appDir); err != nil {
		return err
	}
	return browser.configureApp(
		options,
		filepath.Join(appDir, filepath.FromSlash(browser.Config.MacOS.LauncherPath)),
	)
}
