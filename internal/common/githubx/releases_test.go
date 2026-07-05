package githubx

import "testing"

func TestSplitRepository(t *testing.T) {
	owner, name, err := SplitRepository("cli/cli")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "cli" || name != "cli" {
		t.Fatalf("SplitRepository returned %q/%q", owner, name)
	}
}

func TestSplitRepositoryRejectsInvalidValues(t *testing.T) {
	for _, repository := range []string{"", "owner", "/repo", "owner/"} {
		if _, _, err := SplitRepository(repository); err == nil {
			t.Fatalf("SplitRepository(%q) succeeded", repository)
		}
	}
}
