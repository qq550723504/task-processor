package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"task-processor/internal/batchcapture"
)

func validConfig(t *testing.T) config {
	t.Helper()
	return config{
		QueuePath:     filepath.Join(t.TempDir(), "queue.json"),
		SourceURL:     "https://detail.1688.com/offer/981645030344.html",
		BatchID:       "test-batch",
		ActorID:       "actor-a",
		Organization:  "org-a",
		BrowserPath:   "chrome.exe",
		ExtensionDist: "dist",
		ProfileDir:    "profile",
	}
}

func (c config) args() []string {
	return []string{
		"--queue", c.QueuePath,
		"--url", c.SourceURL,
		"--batch-id", c.BatchID,
		"--actor", c.ActorID,
		"--organization", c.Organization,
		"--browser", c.BrowserPath,
		"--extension", c.ExtensionDist,
		"--profile", c.ProfileDir,
	}
}

func TestValidateRequiresEveryFlag(t *testing.T) {
	cfg := validConfig(t)
	for _, drop := range []string{"queue", "url", "actor", "organization", "browser", "extension", "profile"} {
		broken := cfg
		switch drop {
		case "queue":
			broken.QueuePath = ""
		case "url":
			broken.SourceURL = ""
		case "actor":
			broken.ActorID = ""
		case "organization":
			broken.Organization = ""
		case "browser":
			broken.BrowserPath = ""
		case "extension":
			broken.ExtensionDist = ""
		case "profile":
			broken.ProfileDir = ""
		}
		if err := broken.validate(); err == nil {
			t.Fatalf("validate accepted a config with no %s", drop)
		}
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

// TestValidateAcceptsOnlyOfferPages pins that the command's admission rule matches
// the extension's own pageSource rule, so a URL the executor accepts is one the
// extension will also accept rather than failing later in the browser.
func TestValidateAcceptsOnlyOfferPages(t *testing.T) {
	accepted := []string{
		"https://detail.1688.com/offer/981645030344.html",
		"http://detail.1688.com/offer/1.html",
		"https://detail.1688.com:443/offer/981645030344.html?spm=1",
		"https://detail.1688.com/offer/981645030344.html#detail",
	}
	for _, url := range accepted {
		cfg := validConfig(t)
		cfg.SourceURL = url
		if err := cfg.validate(); err != nil {
			t.Fatalf("%s rejected: %v", url, err)
		}
	}
	rejected := []string{
		"",
		"https://example.com/offer/981645030344.html",
		"https://detail.1688.com/offer/0.html",
		"https://detail.1688.com/offer/.html",
		"https://detail.1688.com/offer/981645030344.htm",
		"https://evil.example/detail.1688.com/offer/981645030344.html",
		"https://detail.1688.com/offer/981645030344.html.evil",
	}
	for _, url := range rejected {
		cfg := validConfig(t)
		cfg.SourceURL = url
		if err := cfg.validate(); err == nil {
			t.Fatalf("%q was accepted", url)
		}
	}
}

func TestEnsureQueueCreatesAnApprovedQueue(t *testing.T) {
	cfg := validConfig(t)
	path, err := ensureQueue(cfg)
	if err != nil {
		t.Fatalf("ensure queue: %v", err)
	}
	if path != cfg.QueuePath {
		t.Fatalf("path %q, want %q", path, cfg.QueuePath)
	}
	queue, err := batchcapture.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !queue.ScopeMatches(cfg.ActorID, cfg.Organization) {
		t.Fatalf("the created queue is not approved for the requested scope: %+v", queue)
	}
	if len(queue.Items) != 1 || queue.Items[0].URL != cfg.SourceURL {
		t.Fatalf("unexpected items: %+v", queue.Items)
	}
	if queue.Items[0].State != batchcapture.ItemQueued {
		t.Fatalf("new item state %q, want queued", queue.Items[0].State)
	}
}

// TestEnsureQueueIsIdempotentForTheSameURL is what keeps a re-run from turning into
// a second submission of the same product.
func TestEnsureQueueIsIdempotentForTheSameURL(t *testing.T) {
	cfg := validConfig(t)
	if _, err := ensureQueue(cfg); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := ensureQueue(cfg); err != nil {
		t.Fatalf("second: %v", err)
	}
	if _, err := ensureQueue(cfg); err != nil {
		t.Fatalf("third: %v", err)
	}
	queue, err := batchcapture.Load(cfg.QueuePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(queue.Items) != 1 {
		t.Fatalf("the same URL produced %d items", len(queue.Items))
	}
}

func TestEnsureQueueAppendsNewURLsWithIncreasingSeq(t *testing.T) {
	cfg := validConfig(t)
	if _, err := ensureQueue(cfg); err != nil {
		t.Fatalf("first: %v", err)
	}
	second := cfg
	second.SourceURL = "https://detail.1688.com/offer/123456789012.html"
	if _, err := ensureQueue(second); err != nil {
		t.Fatalf("second: %v", err)
	}
	queue, err := batchcapture.Load(cfg.QueuePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(queue.Items) != 2 {
		t.Fatalf("items %d, want 2", len(queue.Items))
	}
	if queue.Items[1].Seq <= queue.Items[0].Seq {
		t.Fatalf("sequence did not increase: %+v", queue.Items)
	}
}

// TestEnsureQueueRefusesADifferentScope proves a second run cannot silently
// re-attribute an existing batch to another identity.
func TestEnsureQueueRefusesADifferentScope(t *testing.T) {
	cfg := validConfig(t)
	if _, err := ensureQueue(cfg); err != nil {
		t.Fatalf("first: %v", err)
	}
	other := cfg
	other.Organization = "org-b"
	_, err := ensureQueue(other)
	if !errors.Is(err, batchcapture.ErrScopeMismatch) && !strings.Contains(err.Error(), "approved for") {
		t.Fatalf("err=%v, want a scope refusal", err)
	}
	queue, loadErr := batchcapture.Load(cfg.QueuePath)
	if loadErr != nil {
		t.Fatalf("load: %v", loadErr)
	}
	if queue.ApprovedOrganizationID != cfg.Organization {
		t.Fatalf("the stored approval was rewritten to %q", queue.ApprovedOrganizationID)
	}
}
