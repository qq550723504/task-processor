package listingkit

import "context"

type taskExportServiceConfig struct {
	repo Repository
}

type taskExportService struct {
	repo Repository
}

func newTaskExportService(config taskExportServiceConfig) *taskExportService {
	return &taskExportService{
		repo: config.repo,
	}
}

func (s *taskExportService) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	export, err := buildListingKitExport(task, platform)
	if err != nil {
		return nil, err
	}
	return export, nil
}
