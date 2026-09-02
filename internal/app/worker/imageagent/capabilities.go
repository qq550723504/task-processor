package imageagentworker

import (
	"context"
	"fmt"
	"reflect"

	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"
)

// ProfileResolver is the narrow policy dependency consumed by ImageAgent.
// Implementations resolve only an explicit, fully classified policy key.
type ProfileResolver interface {
	Resolve(imagepolicy.ProfileInput) (imagepolicy.ProductImageProfile, error)
}

// ImageCapabilities is the complete provider-neutral image capability set
// required by the ImageAgent worker.
type ImageCapabilities struct {
	SubjectExtractor        productimage.SubjectExtractor
	WhiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	SceneRenderer           productimage.SceneRenderer
	Reviewer                productimage.Reviewer
	UsageQuoter             productimage.UsageQuoter
	ProfileResolver         ProfileResolver
}

type subjectProvider interface {
	productimage.SubjectExtractor
	productimage.UsageQuoter
}

type whiteBackgroundProvider interface {
	productimage.WhiteBackgroundRenderer
	productimage.UsageQuoter
}

type sceneProvider interface {
	productimage.SceneRenderer
	productimage.UsageQuoter
}

type reviewProvider interface {
	productimage.Reviewer
	productimage.UsageQuoter
}

// providerDependencies contains already configured Integration adapters. It
// intentionally accepts neither the historical application config nor a
// provider SDK so configuration parsing cannot leak into this App boundary.
type providerDependencies struct {
	Subject         subjectProvider
	WhiteBackground whiteBackgroundProvider
	Scene           sceneProvider
	Review          reviewProvider
}

// buildProductionImageCapabilities is completed by the fail-closed production
// provider assembly slice. Keeping the default explicit prevents the worker
// from silently falling back to the retired ProductImage runtime.
func buildProductionImageCapabilities(imageCapabilityRuntime) (ImageCapabilities, error) {
	return ImageCapabilities{}, fmt.Errorf("image agent production providers are not configured")
}

func buildImageCapabilities(deps providerDependencies, resolver ProfileResolver) (ImageCapabilities, error) {
	if nilDependency(resolver) {
		return ImageCapabilities{}, fmt.Errorf("image agent policy resolver is required")
	}

	subject, err := productimage.NewSubjectCapability(deps.Subject)
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("build image agent subject capability: %w", err)
	}
	whiteBackground, err := productimage.NewWhiteBackgroundCapability(deps.WhiteBackground)
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("build image agent white-background capability: %w", err)
	}
	scene, err := productimage.NewSceneCapability(deps.Scene)
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("build image agent scene capability: %w", err)
	}
	reviewer, err := productimage.NewReviewCapability(deps.Review)
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("build image agent review capability: %w", err)
	}
	quoter := &imageUsageQuoter{byOperation: map[string]productimage.UsageQuoter{
		"extract_subject":         deps.Subject,
		"render_white_background": deps.WhiteBackground,
		"render_scene":            deps.Scene,
		"review":                  deps.Review,
	}}

	return ImageCapabilities{
		SubjectExtractor: subject, WhiteBackgroundRenderer: whiteBackground,
		SceneRenderer: scene, Reviewer: reviewer, UsageQuoter: quoter,
		ProfileResolver: resolver,
	}, nil
}

// imageUsageQuoter binds each closed technical operation to the same provider
// object that executes it. Business categories and policy vocabulary never
// participate in this dispatch table.
type imageUsageQuoter struct {
	byOperation map[string]productimage.UsageQuoter
}

func (q *imageUsageQuoter) QuoteUsage(ctx context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	if q == nil {
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	quoter := q.byOperation[request.Operation]
	if nilDependency(quoter) {
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	return quoter.QuoteUsage(ctx, request)
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
