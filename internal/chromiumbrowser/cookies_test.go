package chromiumbrowser

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSetCookieAllowlistReconcilesAllowExceptions(t *testing.T) {
	preferences := map[string]any{
		"profile": map[string]any{
			"content_settings": map[string]any{
				"exceptions": map[string]any{
					"cookies": map[string]any{
						"[*.]keep.example,*": map[string]any{
							"last_modified": "13300000000000000",
							"setting":       json.Number("1"),
						},
						"[*.]old.example": map[string]any{
							"setting": 1,
						},
						"[*.]blocked.example": map[string]any{
							"setting": 2,
						},
						"[*.]session.example": map[string]any{
							"setting": 4,
						},
					},
				},
			},
		},
	}

	SetCookieAllowlist(preferences, []string{"[*.]keep.example", "[*.]new.example"})

	exceptions := preferences["profile"].(map[string]any)["content_settings"].(map[string]any)["exceptions"].(map[string]any)["cookies"].(map[string]any)
	assert.DeepEqual(t, exceptions, map[string]any{
		"[*.]keep.example,*": map[string]any{
			"last_modified": "13300000000000000",
			"setting":       1,
		},
		"[*.]new.example,*": map[string]any{
			"setting": 1,
		},
		"[*.]blocked.example": map[string]any{
			"setting": 2,
		},
		"[*.]session.example": map[string]any{
			"setting": 4,
		},
	})
}

func TestSetCookieAllowlistRemovesNonCanonicalAllowExceptions(t *testing.T) {
	preferences := map[string]any{
		"profile": map[string]any{
			"content_settings": map[string]any{
				"exceptions": map[string]any{
					"cookies": map[string]any{
						"[*.]keep.example": map[string]any{
							"last_modified": "13300000000000000",
							"setting":       1,
						},
						"[*.]old.example": map[string]any{
							"setting": 1,
						},
					},
				},
			},
		},
	}

	SetCookieAllowlist(preferences, []string{"[*.]keep.example"})

	exceptions := preferences["profile"].(map[string]any)["content_settings"].(map[string]any)["exceptions"].(map[string]any)["cookies"].(map[string]any)
	assert.DeepEqual(t, exceptions, map[string]any{
		"[*.]keep.example,*": map[string]any{
			"setting": 1,
		},
	})
}

func TestSetCookieAllowlistRemovesAllAllowExceptionsWhenConfiguredEmpty(t *testing.T) {
	preferences := map[string]any{
		"profile": map[string]any{
			"content_settings": map[string]any{
				"exceptions": map[string]any{
					"cookies": map[string]any{
						"[*.]old.example": map[string]any{
							"setting": 1,
						},
						"[*.]blocked.example": map[string]any{
							"setting": 2,
						},
					},
				},
			},
		},
	}

	SetCookieAllowlist(preferences, []string{})

	exceptions := preferences["profile"].(map[string]any)["content_settings"].(map[string]any)["exceptions"].(map[string]any)["cookies"].(map[string]any)
	assert.DeepEqual(t, exceptions, map[string]any{
		"[*.]blocked.example": map[string]any{
			"setting": 2,
		},
	})
}
