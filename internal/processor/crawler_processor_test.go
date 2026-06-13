package processor

import (
	"testing"

	"task-processor/internal/model"
)

func TestCrawlerProcessorFetchRequestFromTaskPreservesZipcode(t *testing.T) {
	processor := &CrawlerProcessor{}
	task := model.Task{
		TenantID:   10,
		StoreID:    20,
		Region:     "us",
		CategoryID: 30,
		ProductID:  "B001",
		Zipcode:    "10001",
	}

	got := processor.fetchRequestFromTask(task, "amazon")
	if got.Zipcode != "10001" {
		t.Fatalf("Zipcode = %q, want 10001", got.Zipcode)
	}
	if got.Platform != "amazon" || got.ProductID != "B001" || got.TenantID != 10 || got.StoreID != 20 || got.CategoryID != 30 || got.Creator != "crawler-consumer" {
		t.Fatalf("fetch request was not built from task fields: %+v", got)
	}
}

func TestCrawlerProcessorCrawlerPlatformFromTaskMapsLegacyMarketplacePlatform(t *testing.T) {
	processor := &CrawlerProcessor{}

	got := processor.crawlerPlatformFromTask(model.Task{Platform: "shein.crawler"})
	if got != "amazon" {
		t.Fatalf("crawlerPlatformFromTask() = %q, want amazon", got)
	}
}

func TestCrawlerProcessorCrawlerPlatformFromTaskPrefersSourcePlatform(t *testing.T) {
	processor := &CrawlerProcessor{}

	got := processor.crawlerPlatformFromTask(model.Task{
		Platform:       "shein.crawler",
		SourcePlatform: "1688",
	})
	if got != "1688" {
		t.Fatalf("crawlerPlatformFromTask() = %q, want source platform", got)
	}
}
