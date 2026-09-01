package image

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"strings"
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

type whiteBackgroundRendererFunc func(context.Context, RenderRequest) (Candidate, error)

func (f whiteBackgroundRendererFunc) RenderWhiteBackground(ctx context.Context, request RenderRequest) (Candidate, error) {
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

func TestSceneCapabilityAcceptsRemoteAndInlineArtifacts(t *testing.T) {
	const designInlineArtifactMaxBytes = 32 << 20
	require.Equal(t, designInlineArtifactMaxBytes, MaxInlineArtifactBytes)

	t.Run("remote URL", func(t *testing.T) {
		renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
			return []Candidate{{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png")}}, nil
		}))
		require.NoError(t, err)

		got, err := renderer.RenderScene(context.Background(), validSceneRequest())
		require.NoError(t, err)
		require.Equal(t, "https://cdn.example/scene.png", got[0].Asset.URL)
		require.Nil(t, got[0].Asset.Bytes)
	})

	t.Run("bounded inline bytes from local-style handoff", func(t *testing.T) {
		decoded, err := base64.StdEncoding.DecodeString("aW5saW5lLWltYWdl")
		require.NoError(t, err)
		backendOutput := []Candidate{{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", decoded)}}
		request := validSceneRequest()
		request.StyleReferences = []Asset{{
			URL: "https://style.example/local-materialized.png", SourceURL: "https://style-origin.example/a.png",
			SourceAssetID: "style-1", Role: RoleSource, Width: 100, Height: 100, Operations: []string{"source"},
		}}
		renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
			return backendOutput, nil
		}))
		require.NoError(t, err)

		got, err := renderer.RenderScene(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, "image/png", got[0].Asset.MediaType)
		require.Equal(t, decoded, got[0].Asset.Bytes)
		require.Empty(t, got[0].Asset.URL)

		backendOutput[0].Asset.Bytes[0] = 'X'
		require.Equal(t, byte('i'), got[0].Asset.Bytes[0], "inline output must be defensively copied")
	})
}

func TestSceneCapabilityEnforcesInlineArtifactExclusiveBoundary(t *testing.T) {
	const designInlineArtifactMaxBytes = 32 << 20
	exact := make([]byte, designInlineArtifactMaxBytes)
	exact[0] = 1
	over := make([]byte, designInlineArtifactMaxBytes+1)

	for name, candidate := range map[string]Candidate{
		"exact limit": {Asset: validInlineGeneratedAsset(RoleScene, "render_scene", exact)},
		"over limit":  {Asset: validInlineGeneratedAsset(RoleScene, "render_scene", over)},
		"both URL and inline": {
			Asset: func() Asset {
				asset := validInlineGeneratedAsset(RoleScene, "render_scene", []byte("inline"))
				asset.URL = "https://cdn.example/both.png"
				return asset
			}(),
		},
		"neither URL nor inline": {
			Asset: func() Asset {
				asset := validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/none.png")
				asset.URL = ""
				asset.MediaType = ""
				return asset
			}(),
		},
		"inline without media type": {
			Asset: func() Asset {
				asset := validInlineGeneratedAsset(RoleScene, "render_scene", []byte("inline"))
				asset.MediaType = ""
				return asset
			}(),
		},
		"noncanonical media type": {
			Asset: func() Asset {
				asset := validInlineGeneratedAsset(RoleScene, "render_scene", []byte("inline"))
				asset.MediaType = " Image/PNG "
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
			if name == "exact limit" {
				require.NoError(t, err)
				require.Len(t, got[0].Asset.Bytes, designInlineArtifactMaxBytes)
				return
			}
			require.ErrorIs(t, err, ErrOutputValidation)
			require.Nil(t, got)
		})
	}
}

func TestSceneCapabilityRejectsDuplicateInlineArtifacts(t *testing.T) {
	t.Parallel()

	renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
		return []Candidate{
			{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", []byte("same"))},
			{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", append([]byte(nil), []byte("same")...))},
		}, nil
	}))
	require.NoError(t, err)

	got, err := renderer.RenderScene(context.Background(), validSceneRequest())
	require.ErrorIs(t, err, ErrOutputValidation)
	require.Nil(t, got)
}

func TestCapabilitiesRejectInlineSourceAndStyleInputsBeforeDispatch(t *testing.T) {
	t.Parallel()

	for name, request := range map[string]SceneRequest{
		"main source": func() SceneRequest {
			request := validSceneRequest()
			request.Source.Bytes = []byte("not-an-authorized-url-only-source")
			request.Source.MediaType = "image/png"
			return request
		}(),
		"style source": func() SceneRequest {
			request := validSceneRequest()
			request.StyleReferences = []Asset{{
				URL: "https://style.example/reference.png", SourceURL: "https://style-origin.example/reference.png",
				Bytes: []byte("inline-style"), MediaType: "image/png",
				SourceAssetID: "style-1", Role: RoleSource, Width: 100, Height: 100, Operations: []string{"source"},
			}}
			return request
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
				calls++
				return nil, nil
			}))
			require.NoError(t, err)

			_, err = renderer.RenderScene(context.Background(), request)
			require.ErrorIs(t, err, ErrInputInvalid)
			require.Zero(t, calls)
		})
	}
}

func TestReviewCapabilityDefensivelyCopiesInlineCandidateInput(t *testing.T) {
	t.Parallel()

	request := validReviewRequest()
	request.Candidates[0].Asset = validInlineGeneratedAsset(RoleScene, "render_scene", []byte("inline"))
	reviewer, err := NewReviewCapability(reviewerFunc(func(_ context.Context, got ReviewRequest) (Review, error) {
		got.Candidates[0].Asset.Bytes[0] = 'X'
		return Review{Score: 1}, nil
	}))
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, byte('i'), request.Candidates[0].Asset.Bytes[0])
}

func TestCapabilityRejectsGeneratedURLMatchingAnyAuthorizedSource(t *testing.T) {
	t.Parallel()

	t.Run("scene style reference", func(t *testing.T) {
		request := validSceneRequest()
		request.StyleReferences = []Asset{{
			URL: "https://style.example/reference.png", SourceURL: "https://style-origin.example/reference.png",
			SourceAssetID: "style-1", Role: RoleSource, Width: 100, Height: 100, Operations: []string{"source"},
		}}
		candidate := validGeneratedAsset(RoleScene, "render_scene", request.StyleReferences[0].SourceURL)
		renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
			return []Candidate{{Asset: candidate}}, nil
		}))
		require.NoError(t, err)

		got, err := renderer.RenderScene(context.Background(), request)
		require.ErrorIs(t, err, ErrOutputValidation)
		require.Nil(t, got)
	})

	t.Run("review sibling source", func(t *testing.T) {
		main := validSourceAsset()
		sibling := Asset{
			URL: "https://sibling.example/b.png", SourceURL: "https://sibling-origin.example/b.png",
			SourceAssetID: "source-2", Role: RoleSource, Width: 100, Height: 100, Operations: []string{"source"},
		}
		candidate := Candidate{Asset: validGeneratedAsset(RoleScene, "render_scene", sibling.URL)}
		calls := 0
		reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
			calls++
			return Review{Score: 1}, nil
		}))
		require.NoError(t, err)

		_, err = reviewer.Review(context.Background(), ReviewRequest{
			Product: validProductContext(), Sources: []Asset{main, sibling}, Candidates: []Candidate{candidate},
		})
		require.ErrorIs(t, err, ErrInputInvalid)
		require.Zero(t, calls)
	})
}

func TestExtractAndWhiteBackgroundRejectSourcePassThrough(t *testing.T) {
	t.Parallel()

	source := validSourceAsset()
	extractor, err := NewSubjectCapability(subjectExtractorFunc(func(context.Context, ExtractRequest) (Candidate, error) {
		return Candidate{Asset: validGeneratedAsset(RoleSubject, "extract_subject", source.SourceURL)}, nil
	}))
	require.NoError(t, err)
	_, err = extractor.Extract(context.Background(), ExtractRequest{Source: source, Product: validProductContext()})
	require.ErrorIs(t, err, ErrOutputValidation)

	white, err := NewWhiteBackgroundCapability(whiteBackgroundRendererFunc(func(context.Context, RenderRequest) (Candidate, error) {
		return Candidate{Asset: validGeneratedAsset(RoleWhiteBackground, "render_white_background", source.URL)}, nil
	}))
	require.NoError(t, err)
	_, err = white.RenderWhiteBackground(context.Background(), RenderRequest{Source: source, Product: validProductContext()})
	require.ErrorIs(t, err, ErrOutputValidation)
}

func TestReviewCapabilityRejectsOversizedInlineInputBeforeDispatch(t *testing.T) {
	const designInlineArtifactMaxBytes = 32 << 20
	inline := make([]byte, designInlineArtifactMaxBytes+1)
	candidate := Candidate{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", inline)}
	calls := 0
	reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
		calls++
		return Review{Score: 1}, nil
	}))
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), ReviewRequest{
		Product: validProductContext(), Sources: []Asset{validSourceAsset()}, Candidates: []Candidate{candidate},
	})
	require.ErrorIs(t, err, ErrInputInvalid)
	require.Zero(t, calls)
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

func TestEveryCapabilityPrefersCancellationOverConcurrentBackendFailure(t *testing.T) {
	backendFailure := errors.New("transport failed after cancellation")

	t.Run("subject", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		capability, err := NewSubjectCapability(subjectExtractorFunc(func(context.Context, ExtractRequest) (Candidate, error) {
			cancel()
			return Candidate{}, backendFailure
		}))
		require.NoError(t, err)
		_, err = capability.Extract(ctx, ExtractRequest{Source: validSourceAsset(), Product: validProductContext()})
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, backendFailure)
	})

	t.Run("white background", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		capability, err := NewWhiteBackgroundCapability(whiteBackgroundRendererFunc(func(context.Context, RenderRequest) (Candidate, error) {
			cancel()
			return Candidate{}, backendFailure
		}))
		require.NoError(t, err)
		_, err = capability.RenderWhiteBackground(ctx, RenderRequest{Source: validSourceAsset(), Product: validProductContext()})
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, backendFailure)
	})

	t.Run("scene", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		capability, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
			cancel()
			return nil, backendFailure
		}))
		require.NoError(t, err)
		_, err = capability.RenderScene(ctx, validSceneRequest())
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, backendFailure)
	})

	t.Run("review", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		capability, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
			cancel()
			return Review{}, backendFailure
		}))
		require.NoError(t, err)
		_, err = capability.Review(ctx, validReviewRequest())
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, backendFailure)
	})
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

func TestReviewCapabilityBoundsSourcesBeforeCloningOrDispatch(t *testing.T) {
	sources := make([]Asset, 65)
	for index := range sources {
		sources[index] = Asset{
			URL:           "https://source.example/" + string(rune(0x5000+index)) + ".png",
			SourceURL:     "https://origin.example/" + string(rune(0x5000+index)) + ".png",
			SourceAssetID: "source-" + string(rune(0x5000+index)), Role: RoleSource,
			Width: 100, Height: 100, Operations: []string{"source"},
		}
	}
	calls := 0
	reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
		calls++
		return Review{Score: 1}, nil
	}))
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), ReviewRequest{
		Product: validProductContext(), Sources: sources,
		Candidates: []Candidate{{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png")}},
	})
	require.ErrorIs(t, err, ErrInputInvalid)
	require.Zero(t, calls)
}

func TestReviewCapabilityRejectsNonFiniteScoreAndOversizedReasons(t *testing.T) {
	aggregateReasons := make([]string, 128)
	for index := range aggregateReasons {
		aggregateReasons[index] = string(rune(0x6500+index)) + strings.Repeat("x", 600)
	}
	for name, review := range map[string]Review{
		"NaN":               {Score: math.NaN()},
		"positive infinity": {Score: math.Inf(1)},
		"negative infinity": {Score: math.Inf(-1)},
		"too many reasons":  {Score: 1, Reasons: make([]string, 129)},
		"overlong reason":   {Score: 1, Reasons: []string{strings.Repeat("x", 8193)}},
		"aggregate reasons": {Score: 1, Reasons: aggregateReasons},
	} {
		t.Run(name, func(t *testing.T) {
			if name == "too many reasons" {
				for index := range review.Reasons {
					review.Reasons[index] = "reason-" + string(rune(0x6000+index))
				}
			}
			reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
				return review, nil
			}))
			require.NoError(t, err)

			_, err = reviewer.Review(context.Background(), validReviewRequest())
			require.ErrorIs(t, err, ErrOutputValidation)
		})
	}
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
		MediaType:     "image/png",
		SourceURL:     "https://source.example:443/a.png",
		SourceAssetID: "source-1",
		Role:          role,
		Width:         1200,
		Height:        1200,
		Operations:    []string{operation},
	}
}

func validInlineGeneratedAsset(role Role, operation string, content []byte) Asset {
	return Asset{
		Bytes:         content,
		MediaType:     "image/png",
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

func validReviewRequest() ReviewRequest {
	return ReviewRequest{
		Product: validProductContext(),
		Sources: []Asset{validSourceAsset()},
		Candidates: []Candidate{{
			Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png"),
		}},
	}
}
