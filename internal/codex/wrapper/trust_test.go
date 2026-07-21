package wrapper

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/4evy/dotfiles/internal/common/fileutil"
	"github.com/pelletier/go-toml/v2"
)

const testArchiveSubcommand = "archive"

func TestTrustArgsUsesCurrentWorkingDirectory(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, "custom-codex")
	workdir := filepath.Join(home, "work")
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", codexHome)
	if err := os.Mkdir(workdir, fileutil.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workdir)

	extraArgs := []string{"hello"}
	args, err := TrustArgs(extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		codexConfigFlag, projectsTrustOverride([]string{workdir}),
		"hello",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("TrustArgs() = %#v, want %#v", args, want)
	}
	for _, path := range []string{filepath.Join(home, ".codex"), codexHome} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("TrustArgs should not create Codex state at %q: %v", path, err)
		}
	}
}

func TestTrustArgsUsesGitRootAndLastExplicitCwd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	subdir := filepath.Join(repo, "nested")
	other := filepath.Join(home, "other")
	for _, dir := range []string{subdir, other} {
		if err := os.MkdirAll(dir, fileutil.DefaultDirPerm); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, repo, "init")
	t.Chdir(subdir)
	gitRoot := runGitOutput(t, subdir, gitRevParseCommand, gitShowTopLevelFlag)

	args, err := TrustArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		codexConfigFlag, projectsTrustOverride([]string{subdir, gitRoot}),
	}
	if !slices.Equal(args, want) {
		t.Fatalf("TrustArgs() = %#v, want %#v", args, want)
	}

	extraArgs := []string{shortCwdFlag, repo, testArchiveSubcommand, longCwdFlag, other}
	args, err = TrustArgs(extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{
		codexConfigFlag, projectsTrustOverride([]string{other}),
		shortCwdFlag, repo, testArchiveSubcommand, longCwdFlag, other,
	}
	if !slices.Equal(args, want) {
		t.Fatalf("TrustArgs() = %#v, want %#v", args, want)
	}
}

func TestResolveLaunchCwd(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "current", want: cwd},
		{name: "long", args: []string{longCwdFlag, "/tmp/long"}, want: "/tmp/long"},
		{name: "long equals", args: []string{longCwdFlagPrefix + "/tmp/equals"}, want: "/tmp/equals"},
		{name: "short", args: []string{shortCwdFlag, "/tmp/short"}, want: "/tmp/short"},
		{name: "short joined", args: []string{shortCwdFlag + "/tmp/joined"}, want: "/tmp/joined"},
		{
			name: "last scoped value wins",
			args: []string{
				shortCwdFlag,
				"/tmp/root",
				testArchiveSubcommand,
				longCwdFlag,
				"/tmp/archive",
			},
			want: "/tmp/archive",
		},
		{
			name: "separator stops option parsing",
			args: []string{argumentSeparator, shortCwdFlag, "/tmp/prompt"},
			want: cwd,
		},
		{name: "relative", args: []string{longCwdFlag, "nested/../work"}, want: filepath.Join(cwd, "work")},
		{name: "tilde is literal", args: []string{longCwdFlag, "~"}, want: filepath.Join(cwd, "~")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveLaunchCwd(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("resolveLaunchCwd(%q) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

func TestProjectsTrustOverrideQuotesProjectPath(t *testing.T) {
	trustTargets := []string{
		`/tmp/project.with "quotes" and \\slashes`,
		`/tmp/other`,
	}
	var config struct {
		Projects map[string]struct {
			TrustLevel string `toml:"trust_level"`
		} `toml:"projects"`
	}
	if err := toml.Unmarshal([]byte(projectsTrustOverride(trustTargets)), &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Projects) != len(trustTargets) {
		t.Fatalf("projects trust override decoded as %#v", config.Projects)
	}
	for _, trustTarget := range trustTargets {
		if config.Projects[trustTarget].TrustLevel != "trusted" {
			t.Fatalf("projects trust override decoded as %#v", config.Projects)
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}
