package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/localagent"
)

func TestVerificationURLCommandDoesNotInvokeCommandInterpreter(t *testing.T) {
	verificationURI := "https://issuer.example/verify?x=1&calc.exe"
	command := newVerificationURLCommand(verificationURI)
	require.Equal(t, "rundll32.exe", filepath.Base(command.Path))
	require.Equal(t, "url.dll,FileProtocolHandler", command.Args[1])
	require.Equal(t, verificationURI, command.Args[2])
}

func TestParseConfigAcceptsOneShotOfferURL(t *testing.T) {
	cfg, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project", "-url", "https://detail.1688.com/offer/1052008074197.html"})
	require.NoError(t, err)
	require.Equal(t, "https://detail.1688.com/offer/1052008074197.html", cfg.CreateURL)
}

func TestParseConfigRejectsOfferPort(t *testing.T) {
	_, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project", "-url", "https://detail.1688.com:443/offer/1052008074197.html"})
	require.Error(t, err)
}

func TestParseConfigRejectsOfferEmptyQuery(t *testing.T) {
	_, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project", "-url", "https://detail.1688.com/offer/1052008074197.html?"})
	require.Error(t, err)
}

func TestParseConfigRejectsAPIBaseEmptyQuery(t *testing.T) {
	_, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086?", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project"})
	require.Error(t, err)
}

func TestCreatePreparedJobPreparesBeforeCreation(t *testing.T) {
	order := []string{}
	jobs := &fakeJobCreator{order: &order}
	crawler := &fakeJobCrawlerPreparer{order: &order}

	jobID, err := createPreparedJob(context.Background(), jobs, crawler, "https://detail.1688.com/offer/1052008074197.html")
	require.NoError(t, err)
	require.Equal(t, "job-1", jobID)
	require.Equal(t, []string{"prepare", "create"}, order)
}

type fakeJobCreator struct {
	order *[]string
}

func (f *fakeJobCreator) CreateJob(context.Context, string) (localagent.Job, error) {
	*f.order = append(*f.order, "create")
	return localagent.Job{ID: "job-1"}, nil
}

type fakeJobCrawlerPreparer struct {
	order *[]string
}

func (f *fakeJobCrawlerPreparer) Prepare(context.Context) error {
	*f.order = append(*f.order, "prepare")
	return nil
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
