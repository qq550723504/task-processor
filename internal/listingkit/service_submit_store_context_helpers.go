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
