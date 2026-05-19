package listingkit

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinPublishRequestForTask(task *Task, req *GenerateRequest) *sheinpub.BuildRequest {
	if req == nil {
		return &sheinpub.BuildRequest{}
	}
	var ctxIdentity openaiclient.Identity
	if task != nil {
		ctxIdentity = openaiclient.Identity{TenantID: task.TenantID, UserID: task.UserID}
	}
	return &sheinpub.BuildRequest{
		Country:            req.Country,
		Language:           req.Language,
		Text:               req.Text,
		BrandHint:          req.BrandHint,
		TargetCategoryHint: req.TargetCategoryHint,
		SheinStoreID:       req.SheinStoreID,
		Context:            openaiclient.WithIdentity(WithTenantID(context.Background(), ctxIdentity.TenantID), ctxIdentity),
	}
}
