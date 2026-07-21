package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/4evy/dotfiles/internal/chromiumbrowser"
	"github.com/4evy/dotfiles/internal/common/cli"
	"github.com/4evy/dotfiles/internal/helium"
	"github.com/spf13/cobra"
)

const (
	version            = "dev"
	settingsFlag       = "settings"
	inputFlag          = "input"
	applySettingsFlag  = "apply-settings"
	profileDirFlag     = "profile-dir"
	standardInputPath  = "-"
	failureExitCode    = 1
	configureArgCount  = 5
	configureModeArg   = 0
	configureRootArg   = 1
	configureAppDirArg = 2
	configureBinDirArg = 3
	configureFlagsArg  = 4
)

func main() {
	cmd := &cobra.Command{
		Use:   "helium-browser",
		Short: "Configure Helium browser",
	}

	configureOptions := helium.ConfigureOptions{ApplySettings: true}
	configureInput := ""
	configure := &cobra.Command{
		Use:   "configure <macos|linux> <cache-dir> <app-dir> <bin-dir> <flags>",
		Short: "Configure an existing Helium application",
		Args:  cobra.ExactArgs(configureArgCount),
		RunE: func(cmd *cobra.Command, args []string) error {
			configureOptions.Mode = args[configureModeArg]
			configureOptions.Root = args[configureRootArg]
			configureOptions.AppDir = args[configureAppDirArg]
			configureOptions.BinDir = args[configureBinDirArg]
			configureOptions.Flags = args[configureFlagsArg]
			input, err := readApplyInput(configureInput)
			if err != nil {
				return err
			}
			configureOptions.Input = input
			return helium.ConfigureInstalled(configureOptions)
		},
	}
	configure.Flags().
		StringArrayVar(
			&configureOptions.Settings,
			settingsFlag,
			nil,
			"Additional extension settings JSON file",
		)
	configure.Flags().StringVar(
		&configureInput,
		inputFlag,
		"",
		"JSON input file supplied by the caller (- for standard input)",
	)
	configure.Flags().
		BoolVar(
			&configureOptions.ApplySettings,
			applySettingsFlag,
			true,
			"Apply Helium profile settings after install",
		)

	applyExtensionOptions := chromiumbrowser.ApplyOptions{}
	applyExtensionInput := ""
	applyExtensionSettings := &cobra.Command{
		Use:     "apply-extension-settings --profile-dir <Default>",
		Aliases: []string{"extset"},
		Short:   "Apply Helium extension settings",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := readApplyInput(applyExtensionInput)
			if err != nil {
				return err
			}
			applyExtensionOptions.Input = input
			return helium.DefaultBrowser().ApplyExtensionSettings(applyExtensionOptions)
		},
	}
	addSettingsFlags(applyExtensionSettings, &applyExtensionOptions, &applyExtensionInput)

	applyProfileOptions := chromiumbrowser.ApplyOptions{}
	applyProfileInput := ""
	applyProfileSettings := &cobra.Command{
		Use:   "apply-profile-settings --profile-dir <Default>",
		Short: "Apply Helium and Chromium profile settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := readApplyInput(applyProfileInput)
			if err != nil {
				return err
			}
			applyProfileOptions.Input = input
			return helium.DefaultBrowser().ApplyProfileSettings(applyProfileOptions)
		},
	}
	addSettingsFlags(applyProfileSettings, &applyProfileOptions, &applyProfileInput)
	cmd.AddCommand(configure, applyExtensionSettings, applyProfileSettings)

	if err := cli.Execute(context.Background(), cmd, os.Args[1:], version); err != nil {
		os.Exit(failureExitCode)
	}
}

func addSettingsFlags(
	command *cobra.Command,
	options *chromiumbrowser.ApplyOptions,
	inputPath *string,
) {
	command.Flags().StringVar(
		&options.ProfileDir,
		profileDirFlag,
		"",
		"Helium profile directory",
	)
	command.Flags().StringArrayVar(
		&options.Settings,
		settingsFlag,
		nil,
		"Additional extension settings JSON file",
	)
	command.Flags().StringVar(
		inputPath,
		inputFlag,
		"",
		"JSON input file supplied by the caller (- for standard input)",
	)
	_ = command.MarkFlagRequired(profileDirFlag)
}

func readApplyInput(path string) (chromiumbrowser.ApplyInput, error) {
	if path == "" {
		return chromiumbrowser.ApplyInput{}, nil
	}
	var reader io.Reader = os.Stdin
	if path != standardInputPath {
		data, err := os.ReadFile(path)
		if err != nil {
			return chromiumbrowser.ApplyInput{}, fmt.Errorf("read settings input %s: %w", path, err)
		}
		reader = bytes.NewReader(data)
	}
	var input chromiumbrowser.ApplyInput
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return chromiumbrowser.ApplyInput{}, fmt.Errorf("decode settings input %s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return chromiumbrowser.ApplyInput{}, fmt.Errorf("decode settings input %s: %w", path, err)
	}
	return input, nil
}
