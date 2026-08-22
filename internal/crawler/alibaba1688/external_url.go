package alibaba1688

import (
	"net/url"
	"strings"
)

type externalURLPolicy struct {
	requireHTTPS  bool
	allowQuery    bool
	allowFragment bool
}

func isValidExternalURL(raw string, policy externalURLPolicy) bool {
	if strings.ContainsAny(raw, " \t\r\n") {
		return false
	}
	if !policy.allowFragment && strings.Contains(raw, "#") {
		return false
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	if !parsed.IsAbs() || parsed.Hostname() == "" || parsed.User != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if policy.requireHTTPS && parsed.Scheme != "https" {
		return false
	}
	if !policy.allowQuery && parsed.RawQuery != "" {
		return false
	}
	return true
}
