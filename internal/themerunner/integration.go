package themerunner

import (
	"fmt"

	codexwrapper "github.com/4evy/dotfiles/internal/codex/wrapper"
	btoptheme "github.com/4evy/dotfiles/internal/theme/btop"
	codextheme "github.com/4evy/dotfiles/internal/theme/codex"
	helixtheme "github.com/4evy/dotfiles/internal/theme/helix"
	"github.com/4evy/dotfiles/internal/theme/terminal"
)

const (
	btopIntegration  = "btop"
	codexIntegration = "codex"
	helixIntegration = "helix"
)

type argumentPreparer func(terminal.Mode, []string) ([]string, func(), error)

type commandPreparer func(string, []string) (string, []string, error)

type integration struct {
	arguments argumentPreparer
	command   commandPreparer
}

var integrations = map[string]integration{
	btopIntegration:  {arguments: btoptheme.ThemeArgs},
	codexIntegration: {arguments: prepareCodexArgs, command: resolveNodeShebang},
	helixIntegration: {arguments: helixtheme.ThemeArgs},
}

func integrationFor(name string) (integration, error) {
	if name == "" {
		return integration{}, nil
	}
	configured, ok := integrations[name]
	if !ok {
		return integration{}, fmt.Errorf("unknown integration %q", name)
	}
	return configured, nil
}

func (i integration) prepareArgs(args []string) ([]string, func(), error) {
	if i.arguments == nil {
		return args, func() {}, nil
	}
	return i.arguments(terminal.Detect(), args)
}

func (i integration) prepareCommand(executable string, args []string) (string, []string, error) {
	if i.command == nil {
		return executable, args, nil
	}
	return i.command(executable, args)
}

func prepareCodexArgs(mode terminal.Mode, args []string) ([]string, func(), error) {
	filteredArgs, err := codexwrapper.TrustArgs(args)
	if err != nil {
		return nil, nil, err
	}
	return codextheme.ThemeArgs(mode, filteredArgs), func() {}, nil
}
