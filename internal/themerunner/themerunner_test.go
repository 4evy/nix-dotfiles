package themerunner

import (
	"strings"
	"testing"
)

func TestDefaultRunnerManifestIsValid(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	manifest, err := loadRunnerManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Runners) == 0 {
		t.Fatal("default runner manifest is empty")
	}
	if got := strings.Join(ConfiguredProgramNames(), ","); got != "btop,codex,helix" {
		t.Fatalf("configured programs = %q", got)
	}
	for _, name := range []string{"btop", "codex", "helix"} {
		runner, ok, err := configuredRunner(name)
		if err != nil {
			t.Fatal(err)
		}
		if !ok || runner.Integration != name {
			t.Fatalf("%s runner integration = %q, found = %v", name, runner.Integration, ok)
		}
	}
}

func TestIntegrationRegistry(t *testing.T) {
	for _, name := range []string{"btop", "codex", "helix"} {
		if _, err := integrationFor(name); err != nil {
			t.Fatalf("integrationFor(%q): %v", name, err)
		}
	}
	if _, err := integrationFor("unknown"); err == nil {
		t.Fatal("unknown integration was accepted")
	}
}
