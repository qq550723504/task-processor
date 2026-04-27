package api

import (
	"context"
	"errors"

	"task-processor/internal/listingkit"
)

func (s *stubGenerationTaskService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not implemented")
}
