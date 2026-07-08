package amazon

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"

	"github.com/mxschmitt/playwright-go"
)

type failureArtifactPage interface {
	Screenshot(options ...playwright.PageScreenshotOptions) ([]byte, error)
	Content() (string, error)
}

type FailureArtifactInput struct {
	URL        string
	Zipcode    string
	ASIN       string
	Error      string
	ErrorType  string
	Retryable  bool
	InstanceID int
}

type failureArtifactMetadata struct {
	Timestamp      string `json:"timestamp"`
	URL            string `json:"url"`
	Zipcode        string `json:"zipcode,omitempty"`
	ASIN           string `json:"asin,omitempty"`
	InstanceID     int    `json:"instanceId"`
	Failure        string `json:"failure"`
	ErrorType      string `json:"errorType,omitempty"`
	Retryable      bool   `json:"retryable"`
	ScreenshotPath string `json:"screenshotPath,omitempty"`
	HTMLPath       string `json:"htmlPath,omitempty"`
	HTMLTruncated  bool   `json:"htmlTruncated"`
}

type FailureArtifactStore struct {
	enabled      bool
	directory    string
	captureHTML  bool
	maxHTMLBytes int
}

func NewFailureArtifactStore(cfg *config.Config) *FailureArtifactStore {
	if cfg == nil || !cfg.Amazon.FailureArtifacts.Enabled {
		return nil
	}

	return &FailureArtifactStore{
		enabled:      true,
		directory:    cfg.Amazon.FailureArtifacts.Directory,
		captureHTML:  cfg.Amazon.FailureArtifacts.CaptureHTML,
		maxHTMLBytes: cfg.Amazon.FailureArtifacts.MaxHTMLBytes,
	}
}

func (s *FailureArtifactStore) Capture(page failureArtifactPage, input FailureArtifactInput) error {
	if s == nil || !s.enabled {
		return nil
	}

	dir := filepath.Join(s.directory, time.Now().Format("20060102"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create failure artifact dir: %w", err)
	}

	baseName := s.buildBaseName(input)
	metadata := failureArtifactMetadata{
		Timestamp:  time.Now().Format(time.RFC3339Nano),
		URL:        input.URL,
		Zipcode:    input.Zipcode,
		ASIN:       input.ASIN,
		InstanceID: input.InstanceID,
		Failure:    input.Error,
		ErrorType:  input.ErrorType,
		Retryable:  input.Retryable,
	}

	var captureErrors []string

	if page != nil {
		screenshotPath := filepath.Join(dir, baseName+".png")
		screenshot, err := page.Screenshot(playwright.PageScreenshotOptions{
			FullPage: playwright.Bool(true),
		})
		if err != nil {
			captureErrors = append(captureErrors, fmt.Sprintf("capture screenshot: %v", err))
		} else if err := os.WriteFile(screenshotPath, screenshot, 0o644); err != nil {
			captureErrors = append(captureErrors, fmt.Sprintf("write screenshot: %v", err))
		} else {
			metadata.ScreenshotPath = screenshotPath
		}

		if s.captureHTML {
			htmlPath := filepath.Join(dir, baseName+".html")
			content, err := page.Content()
			if err != nil {
				captureErrors = append(captureErrors, fmt.Sprintf("capture html: %v", err))
			} else {
				truncatedContent, truncated := truncateUTF8String(content, s.maxHTMLBytes)
				if err := os.WriteFile(htmlPath, []byte(truncatedContent), 0o644); err != nil {
					captureErrors = append(captureErrors, fmt.Sprintf("write html: %v", err))
				} else {
					metadata.HTMLPath = htmlPath
					metadata.HTMLTruncated = truncated
				}
			}
		}
	}

	metadataPath := filepath.Join(dir, baseName+".json")
	payload, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal failure artifact metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, payload, 0o644); err != nil {
		return fmt.Errorf("write failure artifact metadata: %w", err)
	}

	if len(captureErrors) > 0 {
		return errors.New(strings.Join(captureErrors, "; "))
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("已保存失败样本: %s", metadataPath)
	return nil
}

func (s *FailureArtifactStore) buildBaseName(input FailureArtifactInput) string {
	identity := strings.TrimSpace(input.ASIN)
	if identity == "" {
		h := fnv.New64a()
		_, _ = h.Write([]byte(strings.TrimSpace(strings.ToLower(input.URL))))
		identity = fmt.Sprintf("url-%x", h.Sum64())
	}

	identity = sanitizeArtifactSegment(identity)
	if identity == "" {
		identity = "unknown"
	}

	return fmt.Sprintf(
		"%s-inst-%d-%d",
		identity,
		input.InstanceID,
		time.Now().UnixNano(),
	)
}

func sanitizeArtifactSegment(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func truncateUTF8String(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len([]byte(value)) <= maxBytes {
		return value, false
	}

	var b strings.Builder
	currentBytes := 0
	for _, r := range value {
		runeBytes := len([]byte(string(r)))
		if currentBytes+runeBytes > maxBytes {
			return b.String(), true
		}
		b.WriteRune(r)
		currentBytes += runeBytes
	}

	return b.String(), false
}
