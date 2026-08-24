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

type tenantAllowlistedContextAnalyzer struct {
	inner   productimage.ProductContextAnalyzer
	allowed map[string]struct{}
}

func (a *tenantAllowlistedContextAnalyzer) Analyze(ctx context.Context, source *productimage.SourceBundle) (*productimage.ProductContext, error) {
	if a == nil || a.inner == nil {
		return &productimage.ProductContext{}, nil
	}
	if !tenantAllowed(ctx, a.allowed) {
		// Keep denied/legacy requests on the non-model pipeline; later model stages
		// apply the same tenant gate and degrade to review without provider calls.
		return &productimage.ProductContext{}, nil
	}
	return a.inner.Analyze(ctx, source)
}

func (e *tenantAllowlistedFaithfulEditor) Edit(ctx context.Context, req *productimage.FaithfulEditRequest) (*productimage.FaithfulEditResult, error) {
	if e == nil || e.inner == nil {
		return nil, productimage.NewNoRetryError(fmt.Errorf("faithful editor is not configured"))
	}
	if !tenantAllowed(ctx, e.allowed) {
		return nil, productimage.NewTenantModelAccessDeniedError(productimage.AIIdentityFromContext(ctx).TenantID)
	}
	return e.inner.Edit(ctx, req)
}

type tenantAllowlistedSceneGenerator struct {
	inner   productimage.SceneGenerator
	allowed map[string]struct{}
}

func (g *tenantAllowlistedSceneGenerator) GenerateScene(ctx context.Context, req *productimage.SceneGenerationRequest) (*productimage.SceneGenerationResult, error) {
	if g == nil || g.inner == nil {
		return nil, productimage.NewNoRetryError(fmt.Errorf("scene generator is not configured"))
	}
	if !tenantAllowed(ctx, g.allowed) {
		return nil, productimage.NewTenantModelAccessDeniedError(productimage.AIIdentityFromContext(ctx).TenantID)
	}
	return g.inner.GenerateScene(ctx, req)
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
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" {
		return false
	}
	_, ok := allowed[strings.TrimSpace(identity.TenantID)]
	return ok
}
