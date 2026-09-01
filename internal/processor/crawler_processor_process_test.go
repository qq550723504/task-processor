package processor

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/marketplace/sourceproduct"
	worker "task-processor/internal/platform/workerpool"

	"github.com/sirupsen/logrus"
)

type stubCrawlerProcessorRawJSONClient struct{}

func (stubCrawlerProcessorRawJSONClient) GetRawJsonData(*sourceproduct.RawJsonReq) (*sourceproduct.RawJsonResp, error) {
	return nil, nil
}

func (stubCrawlerProcessorRawJSONClient) CreateRawJsonData(*sourceproduct.RawJsonCreateReq) (int64, error) {
	return 0, nil
}

func TestCrawlerProcessorReturnsErrorInsteadOfPanickingOnNilProduct(t *testing.T) {
	fetcher := sourceproduct.NewProductFetcher(
		stubCrawlerProcessorRawJSONClient{},
		&config.AmazonConfig{Enabled: true},
		nil,
	)
	processor := NewCrawlerProcessor(logrus.New(), fetcher, nil, nil)
	job := worker.WorkerJob{
		TaskID:   1,
		TaskData: `{"id":1,"tenantId":1,"storeId":2,"platform":"amazon.crawler","sourcePlatform":"amazon","region":"us","productId":"B001"}`,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ProcessTask() panicked: %v", r)
		}
	}()

	err := processor.ProcessTask(context.Background(), job)
	if err == nil {
		t.Fatal("ProcessTask() error = nil, want explicit fetch failure")
	}
}
