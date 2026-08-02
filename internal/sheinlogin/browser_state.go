package sheinlogin

import "encoding/json"

func cookieOnlyBrowserState(payload map[string]any) map[string]any {
	cookies := []any{}
	if payload != nil {
		switch value := payload["cookies"].(type) {
		case []any:
			cookies = append(cookies, value...)
		case []map[string]any:
			for _, item := range value {
				cookies = append(cookies, item)
			}
		default:
			// Playwright exposes cookies as []playwright.Cookie rather than []any.
			// Normalize JSON-compatible slices so they keep the same persistent format.
			if raw, err := json.Marshal(value); err == nil {
				var normalized []any
				if json.Unmarshal(raw, &normalized) == nil {
					cookies = append(cookies, normalized...)
				}
			}
		}
	}
	return map[string]any{"cookies": cookies}
}
