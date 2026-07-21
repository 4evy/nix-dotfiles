package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/4evy/dotfiles/internal/common/envx"
	"github.com/4evy/dotfiles/internal/common/fileutil"
)

const (
	defaultRunTimeout     = 30 * time.Minute
	defaultCaptureTimeout = 30 * time.Second
	runTimeoutEnv         = "DOTFILES_PROCESS_RUN_TIMEOUT_SECS"
	captureTimeoutEnv     = "DOTFILES_PROCESS_CAPTURE_TIMEOUT_SECS"
	pathEnvKey            = "PATH"
	decimalRadix          = 10
	uint64BitSize         = 64
	unknownStatusCode     = -1
	successStatusCode     = 0
)

type Output struct {
	StatusCode int
	Success    bool
	Stdout     []byte
	Stderr     []byte
}

type environment struct {
	RunTimeoutSecs     string `env:"DOTFILES_PROCESS_RUN_TIMEOUT_SECS"`
	CaptureTimeoutSecs string `env:"DOTFILES_PROCESS_CAPTURE_TIMEOUT_SECS"`
}

func PathOfWithPath(bin, paths string) (string, bool) {
	if IsPathLike(bin) {
		info, err := os.Stat(bin)
		return bin, err == nil && IsExecutableFile(info)
	}
	for _, dir := range filepath.SplitList(paths) {
		candidate := filepath.Join(dir, ExecutableName(bin))
		info, err := os.Stat(candidate)
		if err == nil && IsExecutableFile(info) {
			return candidate, true
		}
	}
	return "", false
}

func RunInWithEnvAndStdin(cwd string, argv []string, env []string, stdin io.Reader) error {
	if len(argv) == 0 {
		return errors.New("empty command")
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		timeoutFromEnv(runTimeoutEnv, defaultRunTimeout),
	)
	defer cancel()
	cmd := command(ctx, cwd, argv, env)
	cmd.Stdin = stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("command timed out: %s", argv[0])
	}
	if err != nil {
		return fmt.Errorf("command failed: %s: %w", argv[0], err)
	}
	return nil
}

func CaptureWithEnvAndStdin(argv []string, env []string, stdin []byte) (Output, error) {
	if len(argv) == 0 {
		return Output{}, errors.New("empty command")
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		timeoutFromEnv(captureTimeoutEnv, defaultCaptureTimeout),
	)
	defer cancel()
	cmd := command(ctx, "", argv, env)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	statusCode := unknownStatusCode
	success := false
	if err == nil {
		statusCode = successStatusCode
		success = true
	} else {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			statusCode = exitErr.ExitCode()
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return Output{
				StatusCode: statusCode,
				Success:    false,
				Stdout:     stdout.Bytes(),
				Stderr:     stderr.Bytes(),
			}, fmt.Errorf(
				"command timed out: %s",
				argv[0],
			)
	}
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return Output{}, fmt.Errorf("failed to spawn %s: %w", argv[0], err)
		}
	}
	return Output{
		StatusCode: statusCode,
		Success:    success,
		Stdout:     stdout.Bytes(),
		Stderr:     stderr.Bytes(),
	}, nil
}

func ExecutableName(name string) string {
	return name
}

func command(ctx context.Context, cwd string, argv []string, env []string) *exec.Cmd {
	program := argv[0]
	if !IsPathLike(program) {
		for _, envVar := range slices.Backward(env) {
			key, value, ok := strings.Cut(envVar, "=")
			if ok && key == pathEnvKey {
				if path, found := PathOfWithPath(program, value); found {
					program = path
				}
				break
			}
		}
	}
	cmd := exec.CommandContext(ctx, program, argv[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = slices.Concat(os.Environ(), env)
	}
	return cmd
}

func IsPathLike(name string) bool {
	return filepath.IsAbs(name) || strings.Contains(name, "/") ||
		strings.ContainsRune(name, os.PathSeparator)
}

func IsExecutableFile(info os.FileInfo) bool {
	return !info.IsDir() && info.Mode()&fileutil.ExecutablePerm != 0
}

func timeoutFromEnv(name string, fallback time.Duration) time.Duration {
	environment := envx.MustParse[environment]()
	raw := environment.RunTimeoutSecs
	if name == captureTimeoutEnv {
		raw = environment.CaptureTimeoutSecs
	}
	value, err := strconv.ParseUint(raw, decimalRadix, uint64BitSize)
	if err != nil || value == 0 {
		return fallback
	}
	return time.Duration(value) * time.Second
}
