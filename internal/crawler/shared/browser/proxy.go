package browser

import (
	"net/url"
	"strings"

	"github.com/playwright-community/playwright-go"
)

func parseProxyServer(raw string) *playwright.Proxy {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return &playwright.Proxy{Server: raw}
	}
	proxy := &playwright.Proxy{Server: parsed.Scheme + "://" + parsed.Host}
	if parsed.User != nil {
		proxy.Username = playwright.String(parsed.User.Username())
		if password, ok := parsed.User.Password(); ok {
			proxy.Password = playwright.String(password)
		}
	}
	return proxy
}
