package chromiumbrowser

import "testing"

func TestExtensionAliasesExcludeReplacedCatalogEntries(t *testing.T) {
	const sourceID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	browser := Config{
		ExecutableName: "component-browser",
		ExtensionIDAliases: map[string]string{
			sourceID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}.Browser()
	if !browser.extensionInstallExclusions()[sourceID] {
		t.Fatal("replaced catalog extension is not excluded")
	}
}
