package httpapi

import (
	"context"
	"fmt"
	"strings"

	productimage "task-processor/internal/productimage"
)

type tenantAllowlistedFaithfulEditor struct {
	inner   productimage.FaithfulEditor
	allowed map[string]struct{}
}

func (e *tenantAllowlistedFaithfulEditor) Edit(ctx context.Context, req *productimage.FaithfulEditRequest) (*productimage.FaithfulEditResult, error) {
	if e == nil || e.inner == nil {
		return nil, productimage.NewNoRetryError(fmt.Errorf("faithful editor is not configured"))
	}
	if !tenantAllowed(ctx, e.allowed) {
		return nil, productimage.NewNoRetryError(fmt.Errorf("productimage model access denied for tenant %q", productimage.AIIdentityFromContext(ctx).TenantID))
	}
	return e.inner.Edit(ctx, req)
}

type tenantAllowlistedReviewModel struct {
	inner   productimage.ImageReviewModel
	allowed map[string]struct{}
}

func (m *tenantAllowlistedReviewModel) Review(ctx context.Context, req *productimage.ReviewModelRequest) (*productimage.ReviewModelResult, error) {
	if m == nil || m.inner == nil {
		return &productimage.ReviewModelResult{}, nil
	}
	if !tenantAllowed(ctx, m.allowed) {
		return &productimage.ReviewModelResult{
			Decision: &productimage.ReviewDecision{
				NeedsReview: true,
				Reasons:     []string{"productimage model access denied for tenant"},
			},
		}, nil
	}
	return m.inner.Review(ctx, req)
}

func tenantAllowed(ctx context.Context, allowed map[string]struct{}) bool {
	identity := productimage.AIIdentityFromContext(ctx)
	_, ok := allowed[strings.TrimSpace(identity.TenantID)]
	return ok
}
