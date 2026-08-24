package consumer

import (
	"context"
	"testing"

	apptask "task-processor/internal/app/task"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestTaskSubmitterRejectsUnknownCrawlerRouteBeforePublish(t *testing.T) {
	submitter := &TaskSubmitter{adapter: apptask.NewMessageAdapter(), logger: logrus.New()}

	err := submitter.SubmitTask(context.Background(), &model.Task{
		ID:             12345,
		Platform:       "unknown.crawler",
		SourcePlatform: "amazon",
		ProductID:      "B001TEST",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "UNKNOWN_CRAWLER_ROUTE")
}

func TestTaskSubmitterRejectsVariantWithoutExplicitParentSourceBeforePublish(t *testing.T) {
	submitter := &TaskSubmitter{adapter: apptask.NewMessageAdapter(), logger: logrus.New()}

	success, failed := submitter.SubmitVariantTasks(context.Background(), &model.Task{
		ID:        12345,
		Platform:  "shein",
		ProductID: "B001TEST",
	}, []model.Variation{{Asin: "B001VARIANT"}}, "B001TEST")

	if success != 0 || failed != 1 {
		t.Fatalf("expected rejected variant and no publish, success=%d failed=%d", success, failed)
	}
}
