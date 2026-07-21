package wrapper

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	codexConfigFlag     = "-c"
	shortCwdFlag        = "-C"
	longCwdFlag         = "--cd"
	longCwdFlagPrefix   = longCwdFlag + "="
	argumentSeparator   = "--"
	gitCommand          = "git"
	gitWorkingDirFlag   = "-C"
	gitRevParseCommand  = "rev-parse"
	gitShowTopLevelFlag = "--show-toplevel"
)

// TrustArgs prepends ephemeral project-trust overrides for Codex's effective cwd.
func TrustArgs(extraArgs []string) ([]string, error) {
	launchCwd, err := resolveLaunchCwd(extraArgs)
	if err != nil {
		return nil, err
	}

	args := []string{
		codexConfigFlag, projectsTrustOverride(resolveTrustTargets(launchCwd)),
	}
	return append(args, extraArgs...), nil
}

func resolveLaunchCwd(args []string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	override, ok := cwdOverride(args)
	if !ok {
		return cwd, nil
	}
	if !filepath.IsAbs(override) {
		override = filepath.Join(cwd, override)
	}
	return filepath.Clean(override), nil
}

func cwdOverride(args []string) (string, bool) {
	var override string
	var found bool
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == argumentSeparator {
			break
		}
		switch {
		case arg == shortCwdFlag || arg == longCwdFlag:
			if index+1 < len(args) {
				index++
				override = args[index]
				found = true
			}
		case strings.HasPrefix(arg, longCwdFlagPrefix):
			override = strings.TrimPrefix(arg, longCwdFlagPrefix)
			found = true
		case strings.HasPrefix(arg, shortCwdFlag) && len(arg) > len(shortCwdFlag):
			override = strings.TrimPrefix(arg, shortCwdFlag)
			found = true
		}
	}
	return override, found
}

func resolveTrustTargets(cwd string) []string {
	targets := []string{cwd}
	output, err := exec.Command(
		gitCommand,
		gitWorkingDirFlag,
		cwd,
		gitRevParseCommand,
		gitShowTopLevelFlag,
	).Output()
	if root := strings.TrimSpace(string(output)); err == nil && root != "" {
		root = filepath.Clean(root)
		if root != cwd {
			targets = append(targets, root)
		}
	}
	return targets
}

func projectsTrustOverride(trustTargets []string) string {
	projects := make([]string, len(trustTargets))
	for index, trustTarget := range trustTargets {
		projects[index] = strconv.Quote(trustTarget) + "={trust_level=\"trusted\"}"
	}
	return "projects={" + strings.Join(projects, ",") + "}"
}
