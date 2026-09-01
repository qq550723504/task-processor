package image

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sceneRendererFunc func(context.Context, SceneRequest) ([]Candidate, error)

func (f sceneRendererFunc) RenderScene(ctx context.Context, request SceneRequest) ([]Candidate, error) {
	return f(ctx, request)
}

type subjectExtractorFunc func(context.Context, ExtractRequest) (Candidate, error)

func (f subjectExtractorFunc) Extract(ctx context.Context, request ExtractRequest) (Candidate, error) {
	return f(ctx, request)
}

type reviewerFunc func(context.Context, ReviewRequest) (Review, error)

func (f reviewerFunc) Review(ctx context.Context, request ReviewRequest) (Review, error) {
	return f(ctx, request)
}

func TestSceneCapabilityRejectsSourcePassThrough(t *testing.T) {
	t.Parallel()

	source := validSourceAsset()
	for name, candidate := range map[string]Candidate{
		"same URL": {
			Asset: validGeneratedAsset(RoleScene, "extract_scene", source.URL),
		},
		"canonical equivalent source URL": {
			Asset: validGeneratedAsset(RoleScene, "render_scene", "https://source.example/a.png"),
		},
		"pass through operation": {
			Asset: validGeneratedAsset(RoleScene, "pass_through", "https://cdn.example/scene.png"),
		},
		"source role": {
			Asset: validGeneratedAsset(RoleSource, "render_scene", "https://cdn.example/scene.png"),
		},
		"missing generation operation": {
			Asset: func() Asset {
				asset := validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png")
				asset.Operations = nil
				return asset
			}(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
				return []Candidate{candidate}, nil
			}))
			require.NoError(t, err)

			got, err := renderer.RenderScene(context.Background(), validSceneRequest())
			require.ErrorIs(t, err, ErrOutputValidation)
			require.Nil(t, got)
		})
	}
}

func TestSceneCapabilityRejectsEmptyAndDuplicateOutputs(t *testing.T) {
	t.Parallel()

	for name, output := range map[string][]Candidate{
		"nil":   nil,
		"empty": {},
		"duplicate URL": {
			{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png")},
			{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
				return output, nil
			}))
			require.NoError(t, err)

			got, err := renderer.RenderScene(context.Background(), validSceneRequest())
			require.ErrorIs(t, err, ErrOutputValidation)
			require.Nil(t, got)
		})
	}
}

func TestSceneCapabilityDefensivelyCopiesRequestAndCandidates(t *testing.T) {
	t.Parallel()

	request := validSceneRequest()
	backendOutput := []Candidate{{
		Asset:    validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png"),
		Metadata: GenerationMetadata{Values: map[string]string{"seed": "42"}},
	}}
	renderer, err := NewSceneCapability(sceneRendererFunc(func(_ context.Context, got SceneRequest) ([]Candidate, error) {
		got.Product.Attributes["material"] = "mutated"
		got.Source.Operations[0] = "mutated"
		got.Options.StyleReferenceIDs[0] = "mutated"
		return backendOutput, nil
	}))
	require.NoError(t, err)

	candidates, err := renderer.RenderScene(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "steel", request.Product.Attributes["material"])
	require.Equal(t, "source", request.Source.Operations[0])
	require.Equal(t, "style-1", request.Options.StyleReferenceIDs[0])

	backendOutput[0].Asset.Operations[0] = "mutated-after-return"
	backendOutput[0].Metadata.Values["seed"] = "mutated-after-return"
	require.Equal(t, []string{"render_scene"}, candidates[0].Asset.Operations)
	require.Equal(t, "42", candidates[0].Metadata.Values["seed"])
}

func TestSceneCapabilityHonorsCancellationImmediatelyBeforeDispatch(t *testing.T) {
	t.Parallel()

	ctx := &successiveErrorContext{errs: []error{nil, context.Canceled}}
	calls := 0
	renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
		calls++
		return []Candidate{{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png")}}, nil
	}))
	require.NoError(t, err)

	got, err := renderer.RenderScene(ctx, validSceneRequest())
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, got)
	require.Zero(t, calls)
}

func TestSceneCapabilityDiscardsOutputWhenContextCancelsDuringDispatch(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
		cancel()
		return []Candidate{{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png")}}, nil
	}))
	require.NoError(t, err)

	got, err := renderer.RenderScene(ctx, validSceneRequest())
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, got)
}

func TestSubjectCapabilityMapsUnknownBackendFailureToStableDomainError(t *testing.T) {
	t.Parallel()

	backendFailure := errors.New("sdk response code 503")
	extractor, err := NewSubjectCapability(subjectExtractorFunc(func(context.Context, ExtractRequest) (Candidate, error) {
		return Candidate{}, backendFailure
	}))
	require.NoError(t, err)

	_, err = extractor.Extract(context.Background(), ExtractRequest{
		Source:  validSourceAsset(),
		Product: validProductContext(),
	})
	require.ErrorIs(t, err, ErrExternalCapabilityUnavailable)
	require.NotErrorIs(t, err, backendFailure)
}

func TestCapabilityConstructorsRejectNilDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewSceneCapability(nil)
	require.ErrorIs(t, err, ErrInputInvalid)
	_, err = NewSubjectCapability(nil)
	require.ErrorIs(t, err, ErrInputInvalid)
	_, err = NewWhiteBackgroundCapability(nil)
	require.ErrorIs(t, err, ErrInputInvalid)
	_, err = NewReviewCapability(nil)
	require.ErrorIs(t, err, ErrInputInvalid)
}

func TestReviewCapabilityRejectsUnknownCandidateRolesBeforeDispatch(t *testing.T) {
	t.Parallel()

	source := validSourceAsset()
	candidate := Candidate{Asset: validGeneratedAsset(Role("unknown"), "render_scene", "https://cdn.example/scene.png")}
	calls := 0
	reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
		calls++
		return Review{Score: 1}, nil
	}))
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), ReviewRequest{
		Product: validProductContext(), Sources: []Asset{source}, Candidates: []Candidate{candidate},
	})
	require.ErrorIs(t, err, ErrInputInvalid)
	require.Zero(t, calls)
}

func TestSceneCapabilityRejectsResourceExhaustionBeforeDispatch(t *testing.T) {
	t.Parallel()

	request := validSceneRequest()
	request.Product.Attributes = make(map[string]string, 257)
	for i := 0; i < 257; i++ {
		request.Product.Attributes[string(rune(0x1000+i))] = "value"
	}
	calls := 0
	renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
		calls++
		return nil, nil
	}))
	require.NoError(t, err)

	got, err := renderer.RenderScene(context.Background(), request)
	require.ErrorIs(t, err, ErrInputInvalid)
	require.Nil(t, got)
	require.Zero(t, calls)
}

type successiveErrorContext struct {
	context.Context
	errs  []error
	calls int
}

func (c *successiveErrorContext) Deadline() (deadline time.Time, ok bool) { return time.Time{}, false }
func (c *successiveErrorContext) Done() <-chan struct{}                   { return nil }
func (c *successiveErrorContext) Value(any) any                           { return nil }

func (c *successiveErrorContext) Err() error {
	if len(c.errs) == 0 {
		return nil
	}
	index := c.calls
	if index >= len(c.errs) {
		index = len(c.errs) - 1
	}
	c.calls++
	return c.errs[index]
}

func validProductContext() ProductContext {
	return ProductContext{
		ProductKey:  "product-1",
		Title:       "Steel bottle",
		ProductType: "bottle",
		Attributes:  map[string]string{"material": "steel"},
	}
}

func validSourceAsset() Asset {
	return Asset{
		URL:           "https://source.example:443/a.png",
		SourceURL:     "https://origin.example/a.png",
		SourceAssetID: "source-1",
		Role:          RoleSource,
		Width:         1200,
		Height:        1200,
		Operations:    []string{"source"},
	}
}

func validGeneratedAsset(role Role, operation, imageURL string) Asset {
	return Asset{
		URL:           imageURL,
		SourceURL:     "https://source.example:443/a.png",
		SourceAssetID: "source-1",
		Role:          role,
		Width:         1200,
		Height:        1200,
		Operations:    []string{operation},
	}
}

func validSceneRequest() SceneRequest {
	return SceneRequest{
		Source:  validSourceAsset(),
		Product: validProductContext(),
		Options: SceneOptions{StyleReferenceIDs: []string{"style-1"}},
	}
}
