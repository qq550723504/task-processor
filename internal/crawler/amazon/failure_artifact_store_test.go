package amazon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/playwright-community/playwright-go"
)

type fakeFailureArtifactPage struct {
	screenshot []byte
	content    string
}

func (p *fakeFailureArtifactPage) Screenshot(options ...playwright.PageScreenshotOptions) ([]byte, error) {
	return p.screenshot, nil
}

func (p *fakeFailureArtifactPage) Content() (string, error) {
	return p.content, nil
}

func TestFailureArtifactStoreCapture(t *testing.T) {
	tempDir := t.TempDir()
	store := &FailureArtifactStore{
		enabled:      true,
		directory:    tempDir,
		captureHTML:  true,
		maxHTMLBytes: 12,
	}

	err := store.Capture(&fakeFailureArtifactPage{
		screenshot: []byte("png-bytes"),
		content:    "hello世界123456",
	}, FailureArtifactInput{
		URL:        "https://www.amazon.com/dp/B001234567",
		Zipcode:    "10001",
		ASIN:       "B001234567",
		Error:      "页面未准备就绪",
		InstanceID: 7,
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}

	jsonFiles, err := filepath.Glob(filepath.Join(tempDir, "*", "*.json"))
	if err != nil {
		t.Fatalf("Glob json files: %v", err)
	}
	if len(jsonFiles) != 1 {
		t.Fatalf("expected 1 metadata file, got %d", len(jsonFiles))
	}

	payload, err := os.ReadFile(jsonFiles[0])
	if err != nil {
		t.Fatalf("Read metadata: %v", err)
	}

	var metadata failureArtifactMetadata
	if err := json.Unmarshal(payload, &metadata); err != nil {
		t.Fatalf("Unmarshal metadata: %v", err)
	}

	if metadata.URL != "https://www.amazon.com/dp/B001234567" {
		t.Fatalf("unexpected metadata url: %s", metadata.URL)
	}
	if metadata.InstanceID != 7 {
		t.Fatalf("unexpected instance id: %d", metadata.InstanceID)
	}
	if metadata.ScreenshotPath == "" || metadata.HTMLPath == "" {
		t.Fatalf("expected screenshot and html paths to be recorded: %+v", metadata)
	}
	if !metadata.HTMLTruncated {
		t.Fatalf("expected HTML to be truncated")
	}

	screenshotBytes, err := os.ReadFile(metadata.ScreenshotPath)
	if err != nil {
		t.Fatalf("Read screenshot: %v", err)
	}
	if string(screenshotBytes) != "png-bytes" {
		t.Fatalf("unexpected screenshot content: %s", string(screenshotBytes))
	}

	htmlBytes, err := os.ReadFile(metadata.HTMLPath)
	if err != nil {
		t.Fatalf("Read html: %v", err)
	}
	if !strings.HasPrefix(string(htmlBytes), "hello") {
		t.Fatalf("unexpected html prefix: %s", string(htmlBytes))
	}
	if len(htmlBytes) > 12 {
		t.Fatalf("expected truncated html to stay within byte limit, got %d", len(htmlBytes))
	}
}
