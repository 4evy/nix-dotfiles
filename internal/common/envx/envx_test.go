package envx

import "testing"

type testEnvironment struct {
	Count int `env:"DOTFILES_TEST_COUNT"`
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
