package listingkit

import (
	"context"
)

func (s *service) resolveSheinSubmitSettings(ctx context.Context, task *Task) SheinSettings {
	return buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)
}

func (s *service) resolveSheinWarehouseCode(ctx context.Context, task *Task, site string) string {
	return buildSubmitRuntimeContextResolver(s).resolveWarehouseCode(ctx, task, site)
}

func (s *service) resolveSheinStoreID(ctx context.Context, task *Task) (int64, error) {
	return buildSubmitRuntimeContextResolver(s).resolveStoreID(ctx, task)
}

func (s *service) resolveSheinStoreProfile(ctx context.Context, task *Task) (*ListingKitStoreProfile, error) {
	return buildSubmitRuntimeContextResolver(s).resolveStoreProfile(ctx, task)
}

func (s *service) resolveSheinStoreSelection(ctx context.Context, task *Task) (*sheinStoreSelection, error) {
	return buildSubmitRuntimeContextResolver(s).resolveStoreSelection(ctx, task)
}

func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {
	if s == nil || s.repo == nil {
		return "publish", nil
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	if task == nil {
		return "publish", nil
	}
	if action := sheinPreferredSubmitAction(task, buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)); action != "" {
		return action, nil
	}
	return "publish", nil
}
