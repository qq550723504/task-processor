package imagepolicy

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	productimage "task-processor/internal/product/image"

	"github.com/stretchr/testify/require"
)

func TestResolverReturnsPolicyForExactKey(t *testing.T) {
	t.Parallel()

	key := PolicyKey{
		Marketplace:   "marketplace-a",
		Country:       "xx",
		Family:        "family-a",
		SceneCategory: "category-a",
	}
	resolver, err := NewResolver(PolicySet{
		Version: "catalog-v1",
		Policies: []Policy{{
			Key: key,
			Thresholds: Thresholds{
				MainReview:            0.61,
				WhiteBackgroundReview: 0.72,
				WhiteCanvasPenalty:    0.08,
			},
			SceneDefaults: productimage.SceneOptions{
				SceneCategory:     "category-a",
				SceneStyle:        "studio",
				BackgroundTone:    "neutral",
				Composition:       "centered",
				PropsLevel:        "none",
				AudienceHint:      "general",
				CustomSceneHint:   "soft shadow",
				SlotRole:          "scene",
				SlotBrief:         "show the product clearly",
				StyleReferenceIDs: []string{"style-a"},
			},
		}},
	})
	require.NoError(t, err)

	got, err := resolver.Resolve(ProfileInput(key))

	require.NoError(t, err)
	require.Equal(t, ProductImageProfile{
		Key:           key,
		PolicyVersion: "catalog-v1",
		Thresholds: Thresholds{
			MainReview:            0.61,
			WhiteBackgroundReview: 0.72,
			WhiteCanvasPenalty:    0.08,
		},
		SceneDefaults: productimage.SceneOptions{
			SceneCategory:     "category-a",
			SceneStyle:        "studio",
			BackgroundTone:    "neutral",
			Composition:       "centered",
			PropsLevel:        "none",
			AudienceHint:      "general",
			CustomSceneHint:   "soft shadow",
			SlotRole:          "scene",
			SlotBrief:         "show the product clearly",
			StyleReferenceIDs: []string{"style-a"},
		},
	}, got)
}

func TestResolverKeepsDistinctExactPoliciesSeparate(t *testing.T) {
	t.Parallel()

	firstKey := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	secondKey := PolicyKey{Marketplace: "marketplace-b", Country: "yy", Family: "family-b", SceneCategory: "category-b"}
	first := policyForTest(firstKey)
	second := policyForTest(secondKey)
	second.Thresholds = Thresholds{MainReview: 0.81, WhiteBackgroundReview: 0.82, WhiteCanvasPenalty: 0.03}
	second.SceneDefaults.SceneStyle = "lifestyle"
	resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: []Policy{first, second}})
	require.NoError(t, err)

	firstProfile, err := resolver.Resolve(ProfileInput(firstKey))
	require.NoError(t, err)
	secondProfile, err := resolver.Resolve(ProfileInput(secondKey))
	require.NoError(t, err)

	require.Equal(t, Thresholds{MainReview: 0.60, WhiteBackgroundReview: 0.70, WhiteCanvasPenalty: 0.10}, firstProfile.Thresholds)
	require.Equal(t, Thresholds{MainReview: 0.81, WhiteBackgroundReview: 0.82, WhiteCanvasPenalty: 0.03}, secondProfile.Thresholds)
	require.Empty(t, firstProfile.SceneDefaults.SceneStyle)
	require.Equal(t, "lifestyle", secondProfile.SceneDefaults.SceneStyle)
}

func TestNewResolverRejectsEmptyPolicySet(t *testing.T) {
	t.Parallel()

	resolver, err := NewResolver(PolicySet{Version: "catalog-v1"})

	require.Nil(t, resolver)
	require.ErrorIs(t, err, ErrInvalidPolicySet)
}

func TestNewResolverRejectsNonCanonicalPolicyKeys(t *testing.T) {
	t.Parallel()

	tests := map[string]PolicyKey{
		"missing marketplace": {Country: "xx", Family: "family-a", SceneCategory: "category-a"},
		"uppercase country":   {Marketplace: "marketplace-a", Country: "XX", Family: "family-a", SceneCategory: "category-a"},
		"long country":        {Marketplace: "marketplace-a", Country: "xxx", Family: "family-a", SceneCategory: "category-a"},
		"trimmed family":      {Marketplace: "marketplace-a", Country: "xx", Family: " family-a", SceneCategory: "category-a"},
		"dotted category":     {Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category.a"},
	}
	for name, key := range tests {
		name, key := name, key
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resolver, err := NewResolver(PolicySet{
				Version:  "catalog-v1",
				Policies: []Policy{policyForTest(key)},
			})

			require.Nil(t, resolver)
			require.ErrorIs(t, err, ErrInvalidPolicySet)
		})
	}
}

func TestNewResolverRejectsDuplicateExactKeys(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	resolver, err := NewResolver(PolicySet{
		Version:  "catalog-v1",
		Policies: []Policy{policyForTest(key), policyForTest(key)},
	})

	require.Nil(t, resolver)
	require.ErrorIs(t, err, ErrInvalidPolicySet)
}

func TestNewResolverRejectsInvalidVersions(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	for _, version := range []string{"", " catalog-v1", "Catalog-v1", strings.Repeat("v", 129)} {
		version := version
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			resolver, err := NewResolver(PolicySet{Version: version, Policies: []Policy{policyForTest(key)}})

			require.Nil(t, resolver)
			require.ErrorIs(t, err, ErrInvalidPolicySet)
		})
	}
}

func TestNewResolverRejectsNonFiniteOrOutOfRangeThresholds(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	for name, thresholds := range map[string]Thresholds{
		"nan main":           {MainReview: math.NaN(), WhiteBackgroundReview: 0.70, WhiteCanvasPenalty: 0.10},
		"infinite white":     {MainReview: 0.60, WhiteBackgroundReview: math.Inf(1), WhiteCanvasPenalty: 0.10},
		"negative penalty":   {MainReview: 0.60, WhiteBackgroundReview: 0.70, WhiteCanvasPenalty: -0.01},
		"main above maximum": {MainReview: 1.01, WhiteBackgroundReview: 0.70, WhiteCanvasPenalty: 0.10},
	} {
		name, thresholds := name, thresholds
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			policy := policyForTest(key)
			policy.Thresholds = thresholds
			resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: []Policy{policy}})

			require.Nil(t, resolver)
			require.ErrorIs(t, err, ErrInvalidPolicySet)
		})
	}
}

func TestNewResolverRejectsSceneDefaultsForAnotherCategory(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	policy := policyForTest(key)
	policy.SceneDefaults.SceneCategory = "category-b"

	resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: []Policy{policy}})

	require.Nil(t, resolver)
	require.ErrorIs(t, err, ErrInvalidPolicySet)
}

func TestNewResolverRejectsPolicyCountAboveResourceLimit(t *testing.T) {
	t.Parallel()

	policies := make([]Policy, 4097)
	for index := range policies {
		policies[index] = policyForTest(PolicyKey{
			Marketplace:   "marketplace-a",
			Country:       "xx",
			Family:        fmt.Sprintf("family-%d", index),
			SceneCategory: "category-a",
		})
	}

	resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: policies})

	require.Nil(t, resolver)
	require.ErrorIs(t, err, ErrInvalidPolicySet)
}

func TestNewResolverRejectsAggregatePolicyBytesAboveResourceLimit(t *testing.T) {
	t.Parallel()

	policies := make([]Policy, 513)
	for index := range policies {
		policies[index] = policyForTest(PolicyKey{
			Marketplace:   "marketplace-a",
			Country:       "xx",
			Family:        fmt.Sprintf("family-%d", index),
			SceneCategory: "category-a",
		})
		policies[index].SceneDefaults.CustomSceneHint = strings.Repeat("x", 8192)
	}

	resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: policies})

	require.Nil(t, resolver)
	require.ErrorIs(t, err, ErrInvalidPolicySet)
}

func TestResolverRejectsNonCanonicalInputWithoutNormalization(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: []Policy{policyForTest(key)}})
	require.NoError(t, err)

	for name, input := range map[string]ProfileInput{
		"leading space": {Marketplace: " marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"},
		"uppercase":     {Marketplace: "marketplace-a", Country: "XX", Family: "family-a", SceneCategory: "category-a"},
		"empty family":  {Marketplace: "marketplace-a", Country: "xx", SceneCategory: "category-a"},
		"unicode":       {Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "categorKy-a"},
	} {
		name, input := name, input
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			profile, err := resolver.Resolve(input)

			require.Equal(t, ProductImageProfile{}, profile)
			require.ErrorIs(t, err, ErrInvalidProfileInput)
		})
	}
}

func TestResolverDoesNotFallbackAcrossKeyDimensions(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: []Policy{policyForTest(key)}})
	require.NoError(t, err)

	inputs := []ProfileInput{
		{Marketplace: "marketplace-b", Country: "xx", Family: "family-a", SceneCategory: "category-a"},
		{Marketplace: "marketplace-a", Country: "yy", Family: "family-a", SceneCategory: "category-a"},
		{Marketplace: "marketplace-a", Country: "xx", Family: "family-b", SceneCategory: "category-a"},
		{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-b"},
	}
	for _, input := range inputs {
		profile, err := resolver.Resolve(input)
		require.Equal(t, ProductImageProfile{}, profile)
		require.ErrorIs(t, err, ErrPolicyNotFound)
	}
}

func TestResolverDefensivelyOwnsPolicyData(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	policy := policyForTest(key)
	policy.SceneDefaults.StyleReferenceIDs = []string{"style-a"}
	set := PolicySet{Version: "catalog-v1", Policies: []Policy{policy}}
	resolver, err := NewResolver(set)
	require.NoError(t, err)

	set.Policies[0].Thresholds.MainReview = 0.99
	set.Policies[0].SceneDefaults.StyleReferenceIDs[0] = "mutated-input"
	first, err := resolver.Resolve(ProfileInput(key))
	require.NoError(t, err)
	require.Equal(t, 0.60, first.Thresholds.MainReview)
	require.Equal(t, []string{"style-a"}, first.SceneDefaults.StyleReferenceIDs)

	first.SceneDefaults.StyleReferenceIDs[0] = "mutated-output"
	second, err := resolver.Resolve(ProfileInput(key))
	require.NoError(t, err)
	require.Equal(t, []string{"style-a"}, second.SceneDefaults.StyleReferenceIDs)
}

func TestResolverSupportsConcurrentExactReads(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: []Policy{policyForTest(key)}})
	require.NoError(t, err)

	var wait sync.WaitGroup
	errors := make(chan error, 64)
	for range 64 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 100 {
				if _, err := resolver.Resolve(ProfileInput(key)); err != nil {
					errors <- err
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
}

func TestNewResolverRejectsInvalidSceneOptions(t *testing.T) {
	t.Parallel()

	key := PolicyKey{Marketplace: "marketplace-a", Country: "xx", Family: "family-a", SceneCategory: "category-a"}
	policy := policyForTest(key)
	policy.SceneDefaults.StyleReferenceIDs = make([]string, 17)

	resolver, err := NewResolver(PolicySet{Version: "catalog-v1", Policies: []Policy{policy}})

	require.Nil(t, resolver)
	require.ErrorIs(t, err, ErrInvalidPolicySet)
}

func policyForTest(key PolicyKey) Policy {
	return Policy{
		Key: key,
		Thresholds: Thresholds{
			MainReview:            0.60,
			WhiteBackgroundReview: 0.70,
			WhiteCanvasPenalty:    0.10,
		},
		SceneDefaults: productimage.SceneOptions{SceneCategory: key.SceneCategory},
	}
}
