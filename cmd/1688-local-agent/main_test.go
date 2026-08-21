package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseConfigAcceptsOneShotOfferURL(t *testing.T) {
	cfg, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project", "-url", "https://detail.1688.com/offer/1052008074197.html"})
	require.NoError(t, err)
	require.Equal(t, "https://detail.1688.com/offer/1052008074197.html", cfg.CreateURL)
}

func TestParseConfigAcceptsBrowserPath(t *testing.T) {
	cfg, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project", "-browser-path", "C:/Program Files/Google/Chrome/Application/chrome.exe"})
	require.NoError(t, err)
	require.Equal(t, "C:/Program Files/Google/Chrome/Application/chrome.exe", cfg.BrowserPath)
}

func TestParseConfigAcceptsScopesOverride(t *testing.T) {
	cfg, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project", "-scopes", "openid custom-scope"})
	require.NoError(t, err)
	require.Equal(t, "openid custom-scope", cfg.Scopes)
}

func TestParseConfigRejectsSourceAccountAndListingStoreFlags(t *testing.T) {
	_, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project", "-source-account-id", "42"})
	require.Error(t, err)
}
