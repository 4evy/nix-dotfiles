package main

import (
	"context"
	"fmt"
	"os"

	"github.com/4evy/dotfiles/internal/common/cli"
	"github.com/4evy/dotfiles/internal/themerunner"
	"github.com/spf13/cobra"
)

const (
	version         = "dev"
	successExitCode = 0
	failureExitCode = 1
)

func main() {
	code := successExitCode
	cmd := &cobra.Command{
		Use:   "terminal-theme-run",
		Short: "Run terminal theme helpers",
	}
	for _, name := range themerunner.ConfiguredProgramNames() {
		program := name
		child := &cobra.Command{
			Use:                program + " [ARG...]",
			Short:              "Run " + program + " with terminal theme integration",
			DisableFlagParsing: true,
			RunE: func(_ *cobra.Command, args []string) error {
				var err error
				var ok bool
				code, ok, err = themerunner.RunConfigured(program, args)
				if err != nil || ok {
					return err
				}
				code = failureExitCode
				return fmt.Errorf("unknown program %q", program)
			},
		}
		cmd.AddCommand(child)
	}
	if err := cli.Execute(context.Background(), cmd, os.Args[1:], version); err != nil {
		os.Exit(failureExitCode)
	}
	os.Exit(code)
}
