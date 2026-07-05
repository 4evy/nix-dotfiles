package chromiumbrowser

import (
	"encoding/json"
	"strings"
)

const (
	chromiumCookieExceptionsPath = "profile.content_settings.exceptions.cookies"
	chromiumContentSettingAllow  = 1
	chromiumPatternSeparator     = ","
	chromiumWildcardPattern      = "*"
)

func SetCookieAllowlist(preferences map[string]any, patterns []string) {
	if patterns == nil {
		return
	}

	allowed := map[string]struct{}{}
	for _, pattern := range patterns {
		pattern = canonicalCookiePattern(pattern)
		if pattern != "" {
			allowed[pattern] = struct{}{}
		}
	}
	exceptions := NestedObject(preferences, chromiumCookieExceptionsPath)
	for pattern, entry := range exceptions {
		if _, ok := allowed[pattern]; ok || !isCookieAllowException(entry) {
			continue
		}
		delete(exceptions, pattern)
	}

	for pattern := range allowed {
		entry, ok := exceptions[pattern].(map[string]any)
		if !ok {
			entry = map[string]any{}
			exceptions[pattern] = entry
		}
		entry["setting"] = chromiumContentSettingAllow
	}
}

func canonicalCookiePattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.Contains(pattern, chromiumPatternSeparator) {
		return pattern
	}
	return pattern + chromiumPatternSeparator + chromiumWildcardPattern
}

func isCookieAllowException(entry any) bool {
	values, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	return contentSettingInt(values["setting"]) == chromiumContentSettingAllow
}

func contentSettingInt(value any) int {
	switch value := value.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		number, err := value.Int64()
		if err != nil {
			return 0
		}
		return int(number)
	default:
		return 0
	}
}
