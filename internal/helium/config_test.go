package helium

import (
	"testing"
)

func TestLoadConfigRejectsInvalidExtensionID(t *testing.T) {
	_, err := loadConfig([]byte(`
[browser]
executable_name = "helium-browser"

[browser.macos]
app_dir = "/Applications/Helium.app"
launcher_path = "Contents/MacOS/Helium"

[browser.paths.macos]
profile_dir = "/tmp/Default"

[browser.paths.linux]
profile_dir = "/tmp/Default"

[browser.extensions]
refined_github_id = "bad"

`), "test Helium defaults")
	if err == nil {
		t.Fatal("expected invalid extension ID error")
	}
}
