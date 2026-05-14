package sheinlogin

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
		}
	}
	return map[string]any{"cookies": cookies}
}
