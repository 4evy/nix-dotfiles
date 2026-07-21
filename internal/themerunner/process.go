package themerunner

import (
	"errors"
	"os"
	"os/exec"
)

const (
	successExitCode = 0
	failureExitCode = 1
)

func RunInheritEnv(program string, args []string, env []string) (int, error) {
	cmd := exec.Command(program, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(env) > 0 {
		cmd.Env = env
	}
	err := cmd.Run()
	if err == nil {
		return successExitCode, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), nil
	}
	return failureExitCode, err
}
