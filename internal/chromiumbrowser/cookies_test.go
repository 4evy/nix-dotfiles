package chromiumbrowser

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

const (
	profilePreferenceKey          = "profile"
	contentSettingsPreferenceKey  = "content_settings"
	exceptionsPreferenceKey       = "exceptions"
	cookiesPreferenceKey          = "cookies"
	lastModifiedKey               = "last_modified"
	testLastModified              = "13300000000000000"
	testKeepCookiePattern         = "[*.]keep.example"
	testKeepCookieCanonical       = testKeepCookiePattern + ",*"
	testOldCookiePattern          = "[*.]old.example"
	testBlockedCookiePattern      = "[*.]blocked.example"
	testContentSettingBlock       = 2
	testContentSettingSessionOnly = 4
	testContentSettingAllowJSON   = "1"
)

func TestSetCookieAllowlistReconcilesAllowExceptions(t *testing.T) {
	preferences := map[string]any{
		profilePreferenceKey: map[string]any{
			contentSettingsPreferenceKey: map[string]any{
				exceptionsPreferenceKey: map[string]any{
					cookiesPreferenceKey: map[string]any{
						testKeepCookieCanonical: map[string]any{
							lastModifiedKey:           testLastModified,
							chromiumContentSettingKey: json.Number(testContentSettingAllowJSON),
						},
						testOldCookiePattern: map[string]any{
							chromiumContentSettingKey: chromiumContentSettingAllow,
						},
						testBlockedCookiePattern: map[string]any{
							chromiumContentSettingKey: testContentSettingBlock,
						},
						"[*.]session.example": map[string]any{
							chromiumContentSettingKey: testContentSettingSessionOnly,
						},
					},
				},
			},
		},
	}

	SetCookieAllowlist(preferences, []string{testKeepCookiePattern, "[*.]new.example"})

	exceptions := preferences[profilePreferenceKey].(map[string]any)[contentSettingsPreferenceKey].(map[string]any)[exceptionsPreferenceKey].(map[string]any)[cookiesPreferenceKey].(map[string]any)
	assert.DeepEqual(t, exceptions, map[string]any{
		testKeepCookieCanonical: map[string]any{
			lastModifiedKey:           testLastModified,
			chromiumContentSettingKey: chromiumContentSettingAllow,
		},
		"[*.]new.example,*": map[string]any{
			chromiumContentSettingKey: chromiumContentSettingAllow,
		},
		testBlockedCookiePattern: map[string]any{
			chromiumContentSettingKey: testContentSettingBlock,
		},
		"[*.]session.example": map[string]any{
			chromiumContentSettingKey: testContentSettingSessionOnly,
		},
	})
}

func TestSetCookieAllowlistRemovesNonCanonicalAllowExceptions(t *testing.T) {
	preferences := map[string]any{
		profilePreferenceKey: map[string]any{
			contentSettingsPreferenceKey: map[string]any{
				exceptionsPreferenceKey: map[string]any{
					cookiesPreferenceKey: map[string]any{
						testKeepCookiePattern: map[string]any{
							lastModifiedKey:           testLastModified,
							chromiumContentSettingKey: chromiumContentSettingAllow,
						},
						testOldCookiePattern: map[string]any{
							chromiumContentSettingKey: chromiumContentSettingAllow,
						},
					},
				},
			},
		},
	}

	SetCookieAllowlist(preferences, []string{testKeepCookiePattern})

	exceptions := preferences[profilePreferenceKey].(map[string]any)[contentSettingsPreferenceKey].(map[string]any)[exceptionsPreferenceKey].(map[string]any)[cookiesPreferenceKey].(map[string]any)
	assert.DeepEqual(t, exceptions, map[string]any{
		testKeepCookieCanonical: map[string]any{
			chromiumContentSettingKey: chromiumContentSettingAllow,
		},
	})
}

func TestSetCookieAllowlistRemovesAllAllowExceptionsWhenConfiguredEmpty(t *testing.T) {
	preferences := map[string]any{
		profilePreferenceKey: map[string]any{
			contentSettingsPreferenceKey: map[string]any{
				exceptionsPreferenceKey: map[string]any{
					cookiesPreferenceKey: map[string]any{
						testOldCookiePattern: map[string]any{
							chromiumContentSettingKey: chromiumContentSettingAllow,
						},
						testBlockedCookiePattern: map[string]any{
							chromiumContentSettingKey: testContentSettingBlock,
						},
					},
				},
			},
		},
	}

	SetCookieAllowlist(preferences, []string{})

	exceptions := preferences[profilePreferenceKey].(map[string]any)[contentSettingsPreferenceKey].(map[string]any)[exceptionsPreferenceKey].(map[string]any)[cookiesPreferenceKey].(map[string]any)
	assert.DeepEqual(t, exceptions, map[string]any{
		testBlockedCookiePattern: map[string]any{
			chromiumContentSettingKey: testContentSettingBlock,
		},
	})
}
