package zitadelprovision

import (
	"errors"
	"net/url"
	"strconv"
)

func projectCheckOrDefault(value *bool) bool { return value == nil || *value }

func validateLocalApplicationURLs(cfg LocalApplicationConfig) error {
	origin := "http://localhost:3000"
	if cfg.LocalOrigin != "" {
		origin = cfg.LocalOrigin
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawFragment != "" || parsed.Opaque != "" {
			return errors.New("local OIDC origin must be a canonical loopback HTTP origin")
		}
		if parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1" {
			return errors.New("local OIDC origin must be loopback")
		}
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != parsed.Port() || parsed.String() != origin {
			return errors.New("local OIDC origin requires a valid explicit port")
		}
	}
	if !equalStrings(cfg.RedirectURIs, []string{origin + "/api/auth/callback/zitadel"}) || !equalStrings(cfg.PostLogoutRedirectURIs, []string{origin}) {
		return errors.New("local OIDC callback and logout must exactly match the configured local origin")
	}
	return nil
}

func localOIDCGrantTypes(cfg LocalApplicationConfig) []string {
	result := []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"}
	if cfg.EnableRefreshToken {
		result = append(result, "OIDC_GRANT_TYPE_REFRESH_TOKEN")
	}
	return result
}
