package themerunner

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/dotfiles/internal/common/envx"
	commonprocess "github.com/4evy/dotfiles/internal/common/process"
	"github.com/pelletier/go-toml/v2"
)

const (
	environmentVariablePrefix = "$"
	shebangReadSize           = 64
	envShebang                = "#!/usr/bin/env"
	nodeRuntimeName           = "node"
)

//go:embed runtime_defaults.toml
var runtimeDefaultData []byte

var runtimeDefaults = mustLoadRuntimeDefaults(runtimeDefaultData)

type runtimeDefaultsFile struct {
	JavaScriptRuntimes        []string `toml:"javascript_runtimes"`
	JavaScriptRuntimePaths    []string `toml:"javascript_runtime_paths"`
	JavaScriptRuntimeHomePath []string `toml:"javascript_runtime_home_paths"`
}

type executionEnvironment struct {
	Path string `env:"PATH"`
}

func mustLoadRuntimeDefaults(data []byte) runtimeDefaultsFile {
	defaults, err := loadRuntimeDefaults(data)
	if err != nil {
		panic(err)
	}
	return defaults
}

func loadRuntimeDefaults(data []byte) (runtimeDefaultsFile, error) {
	var defaults runtimeDefaultsFile
	if err := toml.Unmarshal(data, &defaults); err != nil {
		return runtimeDefaultsFile{}, fmt.Errorf("parse embedded terminal theme runtime defaults: %w", err)
	}
	if len(defaults.JavaScriptRuntimes) == 0 {
		return runtimeDefaultsFile{}, fmt.Errorf("embedded terminal theme runtime defaults are missing javascript_runtimes")
	}
	for _, name := range defaults.JavaScriptRuntimes {
		if name == "" || commonprocess.IsPathLike(name) {
			return runtimeDefaultsFile{}, fmt.Errorf("embedded terminal theme runtime defaults contain invalid runtime %q", name)
		}
	}
	if slices.Contains(slices.Concat(defaults.JavaScriptRuntimePaths, defaults.JavaScriptRuntimeHomePath), "") {
		return runtimeDefaultsFile{}, fmt.Errorf("embedded terminal theme runtime defaults contain an empty path")
	}
	return defaults, nil
}

func (r runnerSpec) run(extraArgs []string) (int, error) {
	programs := r.Programs
	if len(programs) == 0 {
		programs = []string{r.Name}
	}
	var skip []string
	for _, name := range r.SkipEnv {
		if value := os.Getenv(name); value != "" {
			skip = append(skip, filepath.SplitList(value)...)
		}
	}
	if exe, err := os.Executable(); err == nil {
		skip = append(skip, exe)
	}
	executable := ""
	for _, rawName := range programs {
		name := expandPath(rawName)
		if envName, ok := strings.CutPrefix(rawName, environmentVariablePrefix); ok {
			value := os.Getenv(envName)
			if value == "" {
				continue
			}
			name = value
		}
		var candidates []string
		if commonprocess.IsPathLike(name) {
			info, err := os.Stat(name)
			if err == nil && commonprocess.IsExecutableFile(info) {
				candidates = append(candidates, name)
			}
		} else {
			environment := envx.MustParse[executionEnvironment]()
			for _, dir := range filepath.SplitList(environment.Path) {
				path := filepath.Join(dir, name)
				info, err := os.Stat(path)
				if err == nil && commonprocess.IsExecutableFile(info) {
					candidates = append(candidates, path)
				}
			}
			if len(candidates) == 0 {
				if path, err := exec.LookPath(name); err == nil {
					candidates = append(candidates, path)
				}
			}
		}
		for _, candidate := range candidates {
			matchedSkip := slices.ContainsFunc(skip, func(other string) bool {
				samePath := candidate == other
				sameResolvedPath := false
				if !samePath {
					left, lerr := filepath.EvalSymlinks(candidate)
					right, rerr := filepath.EvalSymlinks(other)
					sameResolvedPath = lerr == nil && rerr == nil && left == right
				}
				return samePath || sameResolvedPath
			})
			if !matchedSkip {
				executable = candidate
				break
			}
		}
		if executable != "" {
			break
		}
	}
	if executable == "" {
		return failureExitCode, fmt.Errorf("%s executable not found", r.Name)
	}
	integration, err := integrationFor(r.Integration)
	if err != nil {
		return failureExitCode, fmt.Errorf("%s: %w", r.Name, err)
	}
	extraArgs, cleanup, err := integration.prepareArgs(extraArgs)
	if err != nil {
		return failureExitCode, err
	}
	defer cleanup()

	args := slices.Clone(r.DefaultArgs)
	args = append(args, extraArgs...)
	executable, args, err = integration.prepareCommand(executable, args)
	if err != nil {
		return failureExitCode, err
	}
	var childEnv []string
	if len(r.Env) > 0 || len(r.EnvUnset) > 0 {
		childEnv = os.Environ()
		for _, name := range r.EnvUnset {
			prefix := name + "="
			childEnv = slices.DeleteFunc(childEnv, func(item string) bool {
				return strings.HasPrefix(item, prefix)
			})
		}
		childEnv = slices.Concat(childEnv, r.Env)
	}
	return RunInheritEnv(executable, args, childEnv)
}

func resolveNodeShebang(executable string, args []string) (string, []string, error) {
	file, err := os.Open(executable)
	if err != nil {
		return executable, args, nil
	}
	defer func() { _ = file.Close() }()

	header := make([]byte, shebangReadSize)
	n, err := file.Read(header)
	if err != nil && n == 0 {
		return executable, args, nil
	}
	firstLine, _, _ := strings.Cut(string(header[:n]), "\n")
	fields := strings.Fields(firstLine)
	if len(fields) < 2 || fields[0] != envShebang || fields[1] != nodeRuntimeName {
		return executable, args, nil
	}

	runtime, err := findJavaScriptRuntime()
	if err != nil {
		return "", nil, err
	}
	return runtime, slices.Concat([]string{executable}, args), nil
}

func findJavaScriptRuntime() (string, error) {
	for _, name := range runtimeDefaults.JavaScriptRuntimes {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	for _, path := range runtimeDefaults.JavaScriptRuntimePaths {
		info, err := os.Stat(path)
		if err == nil && commonprocess.IsExecutableFile(info) {
			return path, nil
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		for _, relativePath := range runtimeDefaults.JavaScriptRuntimeHomePath {
			path := filepath.Join(home, relativePath)
			info, err := os.Stat(path)
			if err == nil && commonprocess.IsExecutableFile(info) {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf(
		"codex requires one of %s, but none were found on PATH or in host system locations",
		strings.Join(runtimeDefaults.JavaScriptRuntimes, ", "),
	)
}
