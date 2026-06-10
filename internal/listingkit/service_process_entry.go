package listingkit

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

func (s *service) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/service_process",
		"task_id":   task.ID,
	})
	return buildListingKitProcessFlow(s).run(ctx, task, log)
}
