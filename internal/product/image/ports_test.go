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

var canonicalURLSink string

var (
	productContextSink ProductContext
	stringSliceSink    []string
	preflightErrorSink error
)

func TestCanonicalHTTPURLNormalizesDNSAndIPIdentities(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		raw  string
		want string
	}{
		"absolute DNS name, casing, non-default port, path and query": {
			raw:  "HTTPS://SOURCE.EXAMPLE.:8443/catalog/../a.png?z=2&a=1",
			want: "https://source.example:8443/a.png?a=1&z=2",
		},
		"IPv6 literal and default port": {
			raw:  "https://[2001:0DB8:0:0:0:0:0:1]:443/catalog/../a.png?z=2&a=1",
			want: "https://[2001:db8::1]/a.png?a=1&z=2",
		},
		"IPv6 literal and non-default port": {
			raw:  "http://[2001:DB8::1]:8080/a.png",
			want: "http://[2001:db8::1]:8080/a.png",
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, testCase.want, canonicalHTTPURL(testCase.raw))
		})
	}
}

func TestCanonicalHTTPURLNormalizesStrictExplicitPorts(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		raw  string
		want string
	}{
		"HTTPS numeric default with leading zeros": {
			raw: "https://source.example:00443/a.png", want: "https://source.example/a.png",
		},
		"HTTP numeric default with leading zeros": {
			raw: "http://source.example:00080/a.png", want: "http://source.example/a.png",
		},
		"DNS non-default with leading zero": {
			raw: "https://source.example:08443/a.png", want: "https://source.example:8443/a.png",
		},
		"IPv6 numeric default with leading zeros": {
			raw: "https://[2001:DB8::1]:00443/a.png", want: "https://[2001:db8::1]/a.png",
		},
		"IPv6 non-default with leading zero": {
			raw: "https://[2001:DB8::1]:08443/a.png", want: "https://[2001:db8::1]:8443/a.png",
		},
		"lowest valid port": {
			raw: "https://source.example:1/a.png", want: "https://source.example:1/a.png",
		},
		"highest valid port": {
			raw: "https://source.example:65535/a.png", want: "https://source.example:65535/a.png",
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, testCase.want, canonicalHTTPURL(testCase.raw))
		})
	}
}

func TestCanonicalHTTPURLRejectsInvalidExplicitPortsAndAuthority(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"DNS trailing colon":       "https://source.example:/a.png",
		"IPv6 trailing colon":      "https://[2001:db8::1]:/a.png",
		"non-numeric port":         "https://source.example:abc/a.png",
		"signed port":              "https://source.example:-443/a.png",
		"explicit zero port":       "https://source.example:000/a.png",
		"port above 65535":         "https://source.example:65536/a.png",
		"userinfo":                 "https://user@source.example/a.png",
		"fragment":                 "https://source.example/a.png#fragment",
		"invalid escaped hostname": "https://source%2Fexample.com/a.png",
	} {
		t.Run(name, func(t *testing.T) {
			require.Empty(t, canonicalHTTPURL(raw))
		})
	}
}

func TestCanonicalHTTPURLFailsClosedOnMalformedQuery(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"semicolon separator": "https://source.example/a.png?first=1;second=2",
		"bad query escape":    "https://source.example/a.png?first=%zz",
	} {
		t.Run(name, func(t *testing.T) {
			require.Empty(t, canonicalHTTPURL(raw))
		})
	}
}

func TestCanonicalHTTPURLDefinesRepeatedQueryKeyOrdering(t *testing.T) {
	t.Parallel()

	first := canonicalHTTPURL("https://source.example/a.png?z=9&a=2&a=1&b=3")
	second := canonicalHTTPURL("https://source.example/a.png?b=3&a=2&a=1&z=9")
	reversedValues := canonicalHTTPURL("https://source.example/a.png?b=3&a=1&a=2&z=9")

	require.Equal(t, "https://source.example/a.png?a=2&a=1&b=3&z=9", first)
	require.Equal(t, first, second, "query key order is not part of canonical identity")
	require.NotEqual(t, first, reversedValues, "repeated values preserve their order")
}

func TestCanonicalHTTPURLRejectsOversizedRawInputBeforeParsing(t *testing.T) {
	const designMaxImageStringBytes = 8 << 10
	oversized := "https://source.example/" + strings.Repeat("x", designMaxImageStringBytes)
	require.Greater(t, len(oversized), designMaxImageStringBytes)

	allocations := testing.AllocsPerRun(100, func() {
		canonicalURLSink = canonicalHTTPURL(oversized)
	})
	require.Empty(t, canonicalURLSink)
	require.Zero(t, allocations, "oversized raw URLs must return before URL parsing allocates")
}

func TestReviewCapabilityRejectsCanonicalIPv6SiblingSourcePassThrough(t *testing.T) {
	t.Parallel()

	main := validSourceAsset()
	sibling := Asset{
		URL: "https://[2001:0DB8:0:0:0:0:0:1]:443/catalog/../b.png?z=2&a=1", SourceURL: "https://ipv6-origin.example/b.png",
		SourceAssetID: "source-2", Role: RoleSource, Width: 100, Height: 100, Operations: []string{"source"},
	}
	candidate := Candidate{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://[2001:db8::1]/b.png?a=1&z=2")}
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
}

func TestEveryCapabilityRejectsCanonicalPortAliasPassThrough(t *testing.T) {
	t.Parallel()

	t.Run("subject numeric default alias", func(t *testing.T) {
		source := validSourceAsset()
		candidate := validGeneratedAsset(RoleSubject, "extract_subject", "https://source.example:0443/a.png")
		extractor, err := NewSubjectCapability(subjectExtractorFunc(func(context.Context, ExtractRequest) (Candidate, error) {
			return Candidate{Asset: candidate}, nil
		}))
		require.NoError(t, err)
		_, err = extractor.Extract(context.Background(), ExtractRequest{Source: source, Product: validProductContext()})
		require.ErrorIs(t, err, ErrOutputValidation)
	})

	t.Run("white background non-default leading zero alias", func(t *testing.T) {
		source := validSourceAsset()
		source.URL = "https://source.example:8443/a.png"
		candidate := validGeneratedAsset(RoleWhiteBackground, "render_white_background", "https://source.example:08443/a.png")
		candidate.SourceURL = source.URL
		renderer, err := NewWhiteBackgroundCapability(whiteBackgroundRendererFunc(func(context.Context, RenderRequest) (Candidate, error) {
			return Candidate{Asset: candidate}, nil
		}))
		require.NoError(t, err)
		_, err = renderer.RenderWhiteBackground(context.Background(), RenderRequest{Source: source, Product: validProductContext()})
		require.ErrorIs(t, err, ErrOutputValidation)
	})

	t.Run("scene style non-default leading zero alias", func(t *testing.T) {
		request := validSceneRequest()
		request.StyleReferences = []Asset{{
			URL: "https://style.example:8443/reference.png", SourceURL: "https://style-origin.example/reference.png",
			SourceAssetID: "style-1", Role: RoleSource, Width: 100, Height: 100, Operations: []string{"source"},
		}}
		candidate := validGeneratedAsset(RoleScene, "render_scene", "https://style.example:08443/reference.png")
		renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
			return []Candidate{{Asset: candidate}}, nil
		}))
		require.NoError(t, err)
		got, err := renderer.RenderScene(context.Background(), request)
		require.ErrorIs(t, err, ErrOutputValidation)
		require.Nil(t, got)
	})

	t.Run("review IPv6 default alias", func(t *testing.T) {
		source := validSourceAsset()
		source.URL = "https://[2001:0DB8:0:0:0:0:0:1]:443/a.png"
		candidate := validGeneratedAsset(RoleScene, "render_scene", "https://[2001:db8::1]:00443/a.png")
		candidate.SourceURL = source.URL
		calls := 0
		reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
			calls++
			return Review{Score: 1}, nil
		}))
		require.NoError(t, err)
		_, err = reviewer.Review(context.Background(), ReviewRequest{
			Product: validProductContext(), Sources: []Asset{source}, Candidates: []Candidate{{Asset: candidate}},
		})
		require.ErrorIs(t, err, ErrInputInvalid)
		require.Zero(t, calls)
	})
}

func TestCapabilityAllowsArtifactAtGenuinelyDifferentPort(t *testing.T) {
	t.Parallel()

	source := validSourceAsset()
	source.URL = "https://source.example:8443/a.png"
	candidate := validGeneratedAsset(RoleSubject, "extract_subject", "https://source.example:8444/a.png")
	candidate.SourceURL = source.URL
	extractor, err := NewSubjectCapability(subjectExtractorFunc(func(context.Context, ExtractRequest) (Candidate, error) {
		return Candidate{Asset: candidate}, nil
	}))
	require.NoError(t, err)
	got, err := extractor.Extract(context.Background(), ExtractRequest{Source: source, Product: validProductContext()})
	require.NoError(t, err)
	require.Equal(t, "https://source.example:8444/a.png", got.Asset.URL)
}

func TestMalformedQueryURLsHaveStableCapabilityErrors(t *testing.T) {
	t.Parallel()

	t.Run("source input prevents dispatch", func(t *testing.T) {
		source := validSourceAsset()
		source.URL = "https://source.example/a.png?first=1;second=2"
		calls := 0
		extractor, err := NewSubjectCapability(subjectExtractorFunc(func(context.Context, ExtractRequest) (Candidate, error) {
			calls++
			return Candidate{}, nil
		}))
		require.NoError(t, err)
		_, err = extractor.Extract(context.Background(), ExtractRequest{Source: source, Product: validProductContext()})
		require.ErrorIs(t, err, ErrInputInvalid)
		require.Zero(t, calls)
	})

	for name, build := range map[string]func(string) error{
		"subject output": func(raw string) error {
			extractor, err := NewSubjectCapability(subjectExtractorFunc(func(context.Context, ExtractRequest) (Candidate, error) {
				return Candidate{Asset: validGeneratedAsset(RoleSubject, "extract_subject", raw)}, nil
			}))
			require.NoError(t, err)
			_, err = extractor.Extract(context.Background(), ExtractRequest{Source: validSourceAsset(), Product: validProductContext()})
			return err
		},
		"white background output": func(raw string) error {
			renderer, err := NewWhiteBackgroundCapability(whiteBackgroundRendererFunc(func(context.Context, RenderRequest) (Candidate, error) {
				return Candidate{Asset: validGeneratedAsset(RoleWhiteBackground, "render_white_background", raw)}, nil
			}))
			require.NoError(t, err)
			_, err = renderer.RenderWhiteBackground(context.Background(), RenderRequest{Source: validSourceAsset(), Product: validProductContext()})
			return err
		},
		"scene output": func(raw string) error {
			renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
				return []Candidate{{Asset: validGeneratedAsset(RoleScene, "render_scene", raw)}}, nil
			}))
			require.NoError(t, err)
			_, err = renderer.RenderScene(context.Background(), validSceneRequest())
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, build("https://cdn.example/a.png?first=1;second=2"), ErrOutputValidation)
		})
	}

	t.Run("review input prevents dispatch", func(t *testing.T) {
		request := validReviewRequest()
		request.Candidates[0].Asset.URL = "https://cdn.example/a.png?first=%zz"
		calls := 0
		reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
			calls++
			return Review{Score: 1}, nil
		}))
		require.NoError(t, err)
		_, err = reviewer.Review(context.Background(), request)
		require.ErrorIs(t, err, ErrInputInvalid)
		require.Zero(t, calls)
	})
}

func TestStringCollectionValidatorsPreflightBeforeCloneAllocation(t *testing.T) {
	const designMaxImageStringBytes = 8 << 10
	overlong := strings.Repeat("x", designMaxImageStringBytes+1)

	t.Run("product attributes", func(t *testing.T) {
		input := ProductContext{ProductKey: "product-1", Attributes: map[string]string{overlong: "value"}}
		allocations := testing.AllocsPerRun(10, func() {
			productContextSink, preflightErrorSink = validateProductContext(input)
		})
		require.ErrorIs(t, preflightErrorSink, ErrInputInvalid)
		require.Equal(t, ProductContext{}, productContextSink)
		require.Zero(t, allocations)
	})

	t.Run("operations", func(t *testing.T) {
		allocations := testing.AllocsPerRun(10, func() {
			stringSliceSink, preflightErrorSink = validateOperations([]string{overlong}, false)
		})
		require.ErrorIs(t, preflightErrorSink, ErrInputInvalid)
		require.Nil(t, stringSliceSink)
		require.Zero(t, allocations)
	})

	t.Run("normalized strings", func(t *testing.T) {
		allocations := testing.AllocsPerRun(10, func() {
			stringSliceSink, preflightErrorSink = normalizedStrings([]string{overlong}, 1)
		})
		require.ErrorIs(t, preflightErrorSink, ErrInputInvalid)
		require.Nil(t, stringSliceSink)
		require.Zero(t, allocations)
	})
}

func TestProductContextRawResourcePreflightIsAllocationFreeAndCanonicalAgnostic(t *testing.T) {
	boundedNonCanonical := ProductContext{
		ProductKey: " product-1 ",
		Attributes: map[string]string{" material ": " steel "},
	}
	allocations := testing.AllocsPerRun(10, func() {
		preflightErrorSink = preflightProductContextResources(boundedNonCanonical)
	})
	require.NoError(t, preflightErrorSink)
	require.Zero(t, allocations, "raw resource preflight must not trim, canonicalize or clone")

	got, err := validateProductContext(boundedNonCanonical)
	require.ErrorIs(t, err, ErrInputInvalid)
	require.Equal(t, ProductContext{}, got)
}

func TestProductContextRejectsRawAggregateBeforeCanonicalPhase(t *testing.T) {
	const designMaxImageStringBytes = 8 << 10
	attributes := make(map[string]string, 8)
	for index := 0; index < 8; index++ {
		attributes[string(rune('a'+index))] = strings.Repeat(string(rune('a'+index)), designMaxImageStringBytes)
	}
	input := ProductContext{ProductKey: " product-1 ", Attributes: attributes}

	allocations := testing.AllocsPerRun(10, func() {
		preflightErrorSink = preflightProductContextResources(input)
	})
	require.ErrorIs(t, preflightErrorSink, ErrInputInvalid)
	require.Zero(t, allocations, "aggregate overflow must stop before canonical checks or map cloning")
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

func TestGeneratedArtifactAggregatePreflightUsesOverflowSafeBudget(t *testing.T) {
	t.Parallel()

	inline := func(content string) Candidate {
		return Candidate{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", []byte(content))}
	}
	remote := Candidate{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/remote.png")}
	for name, testCase := range map[string]struct {
		candidates []Candidate
		wantErr    bool
	}{
		"exact small budget": {candidates: []Candidate{inline("1234"), inline("5678")}},
		"one byte over":      {candidates: []Candidate{inline("1234"), inline("5678"), inline("9")}, wantErr: true},
		"remote does not consume inline budget": {
			candidates: []Candidate{remote, inline("12345678")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := validateGeneratedArtifactAggregate(testCase.candidates, 8)
			if testCase.wantErr {
				require.ErrorIs(t, err, ErrOutputValidation)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCapabilitiesEnforceRealInlineAggregateBoundary(t *testing.T) {
	const designSingleArtifactBytes = 32 << 20
	const designAggregateArtifactBytes = 64 << 20

	firstBytes := make([]byte, designSingleArtifactBytes)
	secondBytes := make([]byte, designSingleArtifactBytes)
	firstBytes[0] = 1
	secondBytes[len(secondBytes)-1] = 2
	exact := []Candidate{
		{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", firstBytes)},
		{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/remote.png")},
		{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", secondBytes)},
	}
	over := append(append([]Candidate(nil), exact...), Candidate{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", []byte{3})})
	require.Equal(t, designAggregateArtifactBytes, len(firstBytes)+len(secondBytes))

	t.Run("scene exact aggregate with mixed remote artifact", func(t *testing.T) {
		renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
			return exact, nil
		}))
		require.NoError(t, err)
		got, err := renderer.RenderScene(context.Background(), validSceneRequest())
		require.NoError(t, err)
		require.Len(t, got, 3)
	})

	t.Run("scene one byte over aggregate", func(t *testing.T) {
		renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
			return over, nil
		}))
		require.NoError(t, err)
		got, err := renderer.RenderScene(context.Background(), validSceneRequest())
		require.ErrorIs(t, err, ErrOutputValidation)
		require.Nil(t, got)
	})

	t.Run("review exact aggregate with mixed remote artifact", func(t *testing.T) {
		calls := 0
		reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
			calls++
			return Review{Score: 1}, nil
		}))
		require.NoError(t, err)
		_, err = reviewer.Review(context.Background(), ReviewRequest{
			Product: validProductContext(), Sources: []Asset{validSourceAsset()}, Candidates: exact,
		})
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	t.Run("review one byte over aggregate before dispatch", func(t *testing.T) {
		calls := 0
		reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
			calls++
			return Review{Score: 1}, nil
		}))
		require.NoError(t, err)
		_, err = reviewer.Review(context.Background(), ReviewRequest{
			Product: validProductContext(), Sources: []Asset{validSourceAsset()}, Candidates: over,
		})
		require.ErrorIs(t, err, ErrInputInvalid)
		require.Zero(t, calls)
	})
}

func TestReviewCapabilityRejectsDuplicateArtifactIdentitiesBeforeDispatch(t *testing.T) {
	t.Parallel()

	for name, candidates := range map[string][]Candidate{
		"same inline content in different slices": {
			{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", []byte("same-content"))},
			{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", append([]byte(nil), []byte("same-content")...))},
		},
		"canonical equivalent remote URLs": {
			{Asset: validGeneratedAsset(RoleScene, "render_scene", "HTTPS://CDN.EXAMPLE.:443/catalog/../scene.png?b=2&a=1")},
			{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png?a=1&b=2")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
				calls++
				return Review{Score: 1}, nil
			}))
			require.NoError(t, err)
			_, err = reviewer.Review(context.Background(), ReviewRequest{
				Product: validProductContext(), Sources: []Asset{validSourceAsset()}, Candidates: candidates,
			})
			require.ErrorIs(t, err, ErrInputInvalid)
			require.Zero(t, calls)
		})
	}
}

func TestReviewCapabilityAcceptsDistinctInlineArtifactIdentities(t *testing.T) {
	t.Parallel()

	calls := 0
	reviewer, err := NewReviewCapability(reviewerFunc(func(context.Context, ReviewRequest) (Review, error) {
		calls++
		return Review{Score: 1}, nil
	}))
	require.NoError(t, err)
	_, err = reviewer.Review(context.Background(), ReviewRequest{
		Product: validProductContext(), Sources: []Asset{validSourceAsset()}, Candidates: []Candidate{
			{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", []byte("first"))},
			{Asset: validInlineGeneratedAsset(RoleScene, "render_scene", []byte("second"))},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestSceneCapabilityRejectsCanonicalEquivalentArtifactIdentities(t *testing.T) {
	t.Parallel()

	renderer, err := NewSceneCapability(sceneRendererFunc(func(context.Context, SceneRequest) ([]Candidate, error) {
		return []Candidate{
			{Asset: validGeneratedAsset(RoleScene, "render_scene", "HTTPS://CDN.EXAMPLE.:443/catalog/../scene.png?b=2&a=1")},
			{Asset: validGeneratedAsset(RoleScene, "render_scene", "https://cdn.example/scene.png?a=1&b=2")},
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
