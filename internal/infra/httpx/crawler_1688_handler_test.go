package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task-processor/internal/crawler/shared"

	"github.com/sirupsen/logrus"
)

func TestCrawler1688HandlerUsesSourceAccountID(t *testing.T) {
	service := &stub1688CrawlerService{}
	handler := NewCrawler1688Handler(service, logrus.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/crawl", strings.NewReader(`{"url":"https://detail.1688.com/offer/3001.html","source_account_id":3001}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.task == nil || service.task.SourceAccountID != 3001 {
		t.Fatalf("task = %+v, want SourceAccountID 3001", service.task)
	}
}

func TestCrawler1688HandlerRejectsLegacySourceStoreID(t *testing.T) {
	service := &stub1688CrawlerService{}
	handler := NewCrawler1688Handler(service, logrus.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/crawl", strings.NewReader(`{"url":"https://detail.1688.com/offer/3001.html","source_store_id":3001}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.task != nil {
		t.Fatalf("task = %+v, want no crawler submission for unknown source_store_id", service.task)
	}
}

func TestCrawler1688HandlerRejectsNegativeSourceAccountID(t *testing.T) {
	service := &stub1688CrawlerService{}
	handler := NewCrawler1688Handler(service, logrus.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/crawl", strings.NewReader(`{"url":"https://detail.1688.com/offer/3001.html","source_account_id":-1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RegisterRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.task != nil {
		t.Fatalf("task = %+v, want no crawler submission for negative source account id", service.task)
	}
}

type stub1688CrawlerService struct {
	task *shared.CrawlerTask
}

func (s *stub1688CrawlerService) SubmitTask(task *shared.CrawlerTask) error {
	s.task = task
	return nil
}
func (s *stub1688CrawlerService) GetTask(string) (*shared.CrawlerResult, error) { return nil, nil }
func (s *stub1688CrawlerService) DeleteTask(string)                             {}
func (s *stub1688CrawlerService) GetAllTasks() []*shared.CrawlerResult          { return nil }
func (s *stub1688CrawlerService) GetStats() map[string]any                      { return map[string]any{} }
func (s *stub1688CrawlerService) IsHealthy() bool                               { return true }
func (s *stub1688CrawlerService) IsReady() bool                                 { return true }
