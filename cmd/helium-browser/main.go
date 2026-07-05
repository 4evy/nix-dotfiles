package main

import (
	"context"
	"os"

	"github.com/4evy/dotfiles/internal/common/cli"
	"github.com/4evy/dotfiles/internal/helium"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	cmd := &cobra.Command{
		Use:   "helium-browser",
		Short: "Configure Helium browser",
	}

	configureOptions := helium.InstallOptions{ApplySettings: true}
	configure := &cobra.Command{
		Use:   "configure <macos|linux> <cache-dir> <app-dir> <bin-dir> <flags>",
		Short: "Configure an existing Helium application",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			configureOptions.Mode = args[0]
			configureOptions.Root = args[1]
			configureOptions.AppDir = args[2]
			configureOptions.BinDir = args[3]
			configureOptions.Flags = args[4]
			return helium.ConfigureInstalled(configureOptions)
		},
	}
	configure.Flags().
		StringArrayVar(&configureOptions.Settings, "settings", nil, "Additional extension settings JSON file")
	configure.Flags().
		StringVar(&configureOptions.SecretsPath, "secrets", "", "SOPS secrets file containing private native Chromium settings")
	configure.Flags().
		BoolVar(&configureOptions.ApplySettings, "apply-settings", true, "Apply Helium profile settings after install")

	applyOptions := helium.ApplyOptions{}
	applySettings := &cobra.Command{
		Use:     "apply-extension-settings --profile-dir <Default>",
		Aliases: []string{"extset"},
		Short:   "Apply Helium extension settings",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(applyOptions.SettingsSource) == 0 {
				sources, err := helium.DefaultSettingsSources()
				if err != nil {
					return err
				}
				applyOptions.SettingsSource = sources
			}
			return helium.ApplyExtensionSettings(applyOptions)
		},
	}
	applySettings.Flags().
		StringVar(&applyOptions.ProfileDir, "profile-dir", "", "Helium profile directory")
	applySettings.Flags().
		StringArrayVar(&applyOptions.Settings, "settings", nil, "Extension settings JSON file")
	applySettings.Flags().
		BoolVar(&applyOptions.GitHubToken, "gh-token", false, "Ask gh for an auth token and store it for Refined GitHub")
	_ = applySettings.MarkFlagRequired("profile-dir")
	cmd.AddCommand(configure, applySettings)

	if err := cli.Execute(context.Background(), cmd, os.Args[1:], version); err != nil {
		os.Exit(1)
	}
}
