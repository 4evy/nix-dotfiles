package envx

import "testing"

type testEnvironment struct {
	Name  string `env:"DOTFILES_TEST_NAME"`
	Count int    `env:"DOTFILES_TEST_COUNT"`
}

func TestParseUsesTypedEnvironmentValues(t *testing.T) {
	t.Setenv("DOTFILES_TEST_NAME", "spectrum")
	t.Setenv("DOTFILES_TEST_COUNT", "3")

	config, err := Parse[testEnvironment]()
	if err != nil {
		t.Fatal(err)
	}
	if config.Name != "spectrum" || config.Count != 3 {
		t.Fatalf("Parse() = %#v, want typed environment values", config)
	}
}

func TestMustParsePanicsOnInvalidTypedValue(t *testing.T) {
	t.Setenv("DOTFILES_TEST_COUNT", "not-an-integer")
	defer func() {
		if recover() == nil {
			t.Fatal("MustParse() did not panic")
		}
	}()

	_ = MustParse[testEnvironment]()
}
