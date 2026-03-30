package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task-processor/internal/crawler/shared"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

type stubAmazonCrawlerService struct {
	product *model.Product
	err     error
	url     string
}

type stubClassifiedCrawlerError struct {
	message   string
	errorType string
	retryable bool
}

func (e *stubClassifiedCrawlerError) Error() string        { return e.message }
func (e *stubClassifiedCrawlerError) ErrorType() string    { return e.errorType }
func (e *stubClassifiedCrawlerError) RetryableError() bool { return e.retryable }

func (s *stubAmazonCrawlerService) SubmitTask(crawlerTask *shared.CrawlerTask) error {
	return nil
}

func (s *stubAmazonCrawlerService) GetTask(taskID string) (*shared.CrawlerResult, error) {
	return nil, nil
}

func (s *stubAmazonCrawlerService) DeleteTask(taskID string) {}

func (s *stubAmazonCrawlerService) GetAllTasks() []*shared.CrawlerResult {
	return nil
}

func (s *stubAmazonCrawlerService) GetStats() map[string]any {
	return map[string]any{}
}

func (s *stubAmazonCrawlerService) IsHealthy() bool {
	return true
}

func (s *stubAmazonCrawlerService) IsReady() bool {
	return true
}

func (s *stubAmazonCrawlerService) FetchProduct(ctx context.Context, url, asin, region, zipcode string) (*model.Product, string, error) {
	return s.product, s.url, s.err
}

func TestCrawlerHandlerFetchProduct(t *testing.T) {
	handler := NewCrawlerHandler(&stubAmazonCrawlerService{
		product: &model.Product{Asin: "B001", Title: "Demo Product"},
		url:     "https://www.amazon.com/dp/B001",
	}, logrus.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/fetch", strings.NewReader(`{"asin":"B001","region":"us"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp JSON
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success response, got %+v", resp)
	}
}

func TestCrawlerHandlerFetchProductRequiresURLOrASIN(t *testing.T) {
	handler := NewCrawlerHandler(&stubAmazonCrawlerService{}, logrus.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/fetch", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCrawlerHandlerFetchProductReturnsClassifiedError(t *testing.T) {
	handler := NewCrawlerHandler(&stubAmazonCrawlerService{
		err: &stubClassifiedCrawlerError{
			message:   errors.New("navigation timeout exceeded").Error(),
			errorType: "timeout",
			retryable: true,
		},
	}, logrus.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/fetch", strings.NewReader(`{"asin":"B001","region":"us"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp JSON
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Code != "timeout" {
		t.Fatalf("expected code %s, got %s", "timeout", resp.Code)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %#v", resp.Data)
	}
	if retryable, ok := data["retryable"].(bool); !ok || !retryable {
		t.Fatalf("expected retryable=true, got %#v", data["retryable"])
	}
}

func TestCrawlerHandlerFetchProductReturnsTooManyRequestsForSystemBusy(t *testing.T) {
	handler := NewCrawlerHandler(&stubAmazonCrawlerService{
		err: &stubClassifiedCrawlerError{
			message:   "crawler concurrency acquire timeout",
			errorType: "system_busy",
			retryable: true,
		},
	}, logrus.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/fetch", strings.NewReader(`{"asin":"B001","region":"us"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp JSON
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != "system_busy" {
		t.Fatalf("expected code system_busy, got %s", resp.Code)
	}
}
