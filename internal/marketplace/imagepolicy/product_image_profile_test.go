package imagepolicy

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	productimage "task-processor/internal/product/image"

	"github.com/stretchr/testify/require"
)

func TestResolveProductImageProfileRequiresExplicitMarketplaceContext(t *testing.T) {
	t.Parallel()

	_, err := ResolveProductImageProfile(ProfileInput{Country: "us", ProductType: "shoe"})
	require.ErrorIs(t, err, ErrInvalidProfileInput)
	_, err = ResolveProductImageProfile(ProfileInput{Marketplace: "amazon", ProductType: "shoe"})
	require.ErrorIs(t, err, ErrInvalidProfileInput)
}

func TestResolveProductImageProfileRejectsNonCanonicalMarketplaceAndCountry(t *testing.T) {
	t.Parallel()

	tests := []ProfileInput{
		{Marketplace: "ama zon", Country: "us"},
		{Marketplace: "amazon.com", Country: "us"},
		{Marketplace: "amazon", Country: "usa"},
		{Marketplace: "amazon", Country: "u_s"},
		{Marketplace: string([]byte{'a', 0xff}), Country: "us"},
		{Marketplace: "amazon", Country: string([]byte{'u', 0xff})},
	}
	for _, input := range tests {
		_, err := ResolveProductImageProfile(input)
		require.ErrorIs(t, err, ErrInvalidProfileInput, "%+v", input)
	}
}

func TestResolveProductImageProfileRejectsOversizedRawInputBeforeNormalization(t *testing.T) {
	t.Parallel()

	profile, err := ResolveProductImageProfile(ProfileInput{
		Marketplace: "amazon",
		Country:     "us",
		ProductType: strings.Repeat("x", 8192),
	})
	require.NoError(t, err)
	require.Equal(t, "default", profile.Family)

	_, err = ResolveProductImageProfile(ProfileInput{
		Marketplace: "amazon",
		Country:     "us",
		ProductType: strings.Repeat("A", 8193),
	})
	require.ErrorIs(t, err, ErrInvalidProfileInput)

	_, err = ResolveProductImageProfile(ProfileInput{
		Marketplace: "amazon",
		Country:     "us",
		ProductType: strings.Repeat(" ", 8193),
	})
	require.ErrorIs(t, err, ErrInvalidProfileInput)
}

func TestResolveProductImageProfileEnforcesAggregateRawInputBoundary(t *testing.T) {
	t.Parallel()

	_, err := ResolveProductImageProfile(ProfileInput{
		Marketplace:   "amazon",
		Country:       "us",
		ProductType:   strings.Repeat("x", 8192),
		SceneCategory: strings.Repeat(" ", 8179) + "shoes",
	})
	require.NoError(t, err, "fixture is exactly 16384 raw bytes")

	_, err = ResolveProductImageProfile(ProfileInput{
		Marketplace:   "amazon",
		Country:       "us",
		ProductType:   strings.Repeat("x", 8192),
		SceneCategory: strings.Repeat(" ", 8180) + "shoes",
	})
	require.ErrorIs(t, err, ErrInvalidProfileInput, "fixture is 16385 raw bytes")
}

func TestResolveProductImageProfileSelectsAmazonUSFamilyDeterministically(t *testing.T) {
	t.Parallel()

	for _, productType := range []string{"shoe ring", "ring shoe"} {
		for i := 0; i < 100; i++ {
			profile, err := ResolveProductImageProfile(ProfileInput{
				Marketplace: " AMAZON ",
				Country:     " US ",
				ProductType: productType,
			})
			require.NoError(t, err)
			require.Equal(t, "footwear", profile.Family)
			require.Equal(t, 0.61, profile.MainReviewThreshold)
			require.Equal(t, 0.68, profile.WhiteBackgroundReviewThreshold)
			require.Equal(t, 0.04, profile.WhiteCanvasPenalty)
		}
	}
}

func TestResolveProductImageProfilePreservesAmazonUSFamilyThresholdContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		productType string
		family      string
		main        float64
		white       float64
		penalty     float64
	}{
		{productType: "running shoe", family: "footwear", main: 0.61, white: 0.68, penalty: 0.04},
		{productType: "winter jacket", family: "apparel", main: 0.62, white: 0.68, penalty: 0.05},
		{productType: "leather handbag", family: "bags_accessories", main: 0.63, white: 0.69, penalty: 0.06},
		{productType: "cotton blanket", family: "home_textiles", main: 0.63, white: 0.69, penalty: 0.06},
		{productType: "bluetooth speaker", family: "electronics", main: 0.69, white: 0.75, penalty: 0.12},
		{productType: "silver necklace", family: "jewelry_watch", main: 0.70, white: 0.76, penalty: 0.14},
		{productType: "perfume bottle", family: "beauty_bottle", main: 0.68, white: 0.74, penalty: 0.12},
	}
	for _, testCase := range tests {
		profile, err := ResolveProductImageProfile(ProfileInput{Marketplace: "amazon", Country: "us", ProductType: testCase.productType})
		require.NoError(t, err)
		require.Equal(t, testCase.family, profile.Family)
		require.Equal(t, testCase.main, profile.MainReviewThreshold)
		require.Equal(t, testCase.white, profile.WhiteBackgroundReviewThreshold)
		require.Equal(t, testCase.penalty, profile.WhiteCanvasPenalty)
	}
}

func TestResolveProductImageProfileDoesNotClassifySubstringFalsePositives(t *testing.T) {
	t.Parallel()

	for _, productType := range []string{"capacity planner", "escape room kit"} {
		profile, err := ResolveProductImageProfile(ProfileInput{
			Marketplace: "amazon", Country: "us", ProductType: productType,
		})
		require.NoError(t, err)
		require.Equal(t, "default", profile.Family, productType)
	}

	for _, productType := range []string{"handbag", "backpacks", "running shoes"} {
		profile, err := ResolveProductImageProfile(ProfileInput{
			Marketplace: "amazon", Country: "us", ProductType: productType,
		})
		require.NoError(t, err)
		require.NotEqual(t, "default", profile.Family, productType)
	}
}

func TestResolveProductImageProfileAcceptsOnlyExplicitFamilyLexemes(t *testing.T) {
	t.Parallel()

	accepted := []struct {
		productType string
		family      string
	}{
		{productType: "caps", family: "bags_accessories"},
		{productType: "shoes", family: "footwear"},
		{productType: "dresses", family: "apparel"},
		{productType: "handbags", family: "bags_accessories"},
		{productType: "backpacks", family: "bags_accessories"},
		{productType: "pants", family: "apparel"},
		{productType: "electronics", family: "electronics"},
	}
	for _, testCase := range accepted {
		profile, err := ResolveProductImageProfile(ProfileInput{Marketplace: "amazon", Country: "us", ProductType: testCase.productType})
		require.NoError(t, err)
		require.Equal(t, testCase.family, profile.Family, testCase.productType)
	}

	for _, productType := range []string{"capes", "shoees", "dresss", "handbages", "backpackes", "pantss", "electronicss"} {
		profile, err := ResolveProductImageProfile(ProfileInput{Marketplace: "amazon", Country: "us", ProductType: productType})
		require.NoError(t, err)
		require.Equal(t, "default", profile.Family, productType)
	}
}

func TestResolveProductImageProfileTreatsCombiningMarksAsTokenCharacters(t *testing.T) {
	t.Parallel()

	profile, err := ResolveProductImageProfile(ProfileInput{
		Marketplace: "amazon", Country: "us", ProductType: "cap\u0301acity",
	})
	require.NoError(t, err)
	require.Equal(t, "default", profile.Family)
}

func TestResolveProductImageProfileFamilyPriorityIsIndependentOfTokenOrder(t *testing.T) {
	t.Parallel()

	productTypes := []string{"shoe ring", "ring shoe", "shoe-ring", "ring/shoe"}
	for range 100 {
		for _, productType := range productTypes {
			profile, err := ResolveProductImageProfile(ProfileInput{Marketplace: "amazon", Country: "us", ProductType: productType})
			require.NoError(t, err)
			require.Equal(t, "footwear", profile.Family, productType)
		}
	}
}

func TestResolveProductImageProfileUsesGenericThresholdsOutsideAmazonUS(t *testing.T) {
	t.Parallel()

	profile, err := ResolveProductImageProfile(ProfileInput{
		Marketplace: "amazon", Country: "ca", ProductType: "running shoes",
	})
	require.NoError(t, err)
	require.Equal(t, "default", profile.Family)
	require.Equal(t, 0.65, profile.MainReviewThreshold)
	require.Equal(t, 0.70, profile.WhiteBackgroundReviewThreshold)
	require.Equal(t, 0.10, profile.WhiteCanvasPenalty)
}

func TestResolveProductImageProfileAllowsExplicitUnknownMarketplaceWithoutPlatformDefaults(t *testing.T) {
	t.Parallel()

	profile, err := ResolveProductImageProfile(ProfileInput{
		Marketplace: " Etsy ", Country: " US ", ProductType: "running shoes",
	})
	require.NoError(t, err)
	require.Equal(t, "etsy", profile.Marketplace)
	require.Equal(t, "us", profile.Country)
	require.Equal(t, "default", profile.Family)
	require.Equal(t, "none", profile.SceneDefaultsSource)
	require.Equal(t, productimage.SceneOptions{}, profile.SceneDefaults)
}

func TestResolveProductImageProfileKeepsMarketplacePolicyOutOfProductDomain(t *testing.T) {
	t.Parallel()

	profile, err := ResolveProductImageProfile(ProfileInput{
		Marketplace:   "amazon",
		Country:       "us",
		ProductType:   "running sneaker",
		SceneCategory: "shoes",
	})
	require.NoError(t, err)
	require.Equal(t, "platform_category", profile.SceneDefaultsSource)
	require.Equal(t, productimage.SceneOptions{
		SceneCategory:  "shoes",
		SceneStyle:     "studio",
		BackgroundTone: "bright",
		Composition:    "centered",
		PropsLevel:     "none",
		AudienceHint:   "premium",
	}, profile.SceneDefaults)
}

func TestResolveProductImageProfileSelectsExplicitDerivedAndPlatformSceneDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      ProfileInput
		wantSource string
		want       productimage.SceneOptions
	}{
		{
			name:       "explicit category overrides derived category",
			input:      ProfileInput{Marketplace: "amazon", Country: "us", ProductType: "running shoe", SceneCategory: " JEWELRY "},
			wantSource: "platform_category",
			want:       productimage.SceneOptions{SceneCategory: "jewelry", SceneStyle: "studio", BackgroundTone: "cool", Composition: "close_up", PropsLevel: "none", AudienceHint: "premium"},
		},
		{
			name:       "category derived from compound product type",
			input:      ProfileInput{Marketplace: "shein", Country: "gb", ProductType: "leather handbag"},
			wantSource: "platform_category",
			want:       productimage.SceneOptions{SceneCategory: "bags", SceneStyle: "lifestyle", BackgroundTone: "warm", Composition: "multi_angle", PropsLevel: "light", AudienceHint: "youthful"},
		},
		{
			name:       "marketplace default for unknown category",
			input:      ProfileInput{Marketplace: "walmart", Country: "ca", ProductType: "desk lamp"},
			wantSource: "platform",
			want:       productimage.SceneOptions{SceneStyle: "lifestyle", BackgroundTone: "neutral", Composition: "centered", PropsLevel: "light", AudienceHint: "homey"},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			profile, err := ResolveProductImageProfile(testCase.input)
			require.NoError(t, err)
			require.Equal(t, testCase.wantSource, profile.SceneDefaultsSource)
			require.Equal(t, testCase.want, profile.SceneDefaults)
		})
	}
}

func TestResolveProductImageProfileRejectsUnsupportedExplicitSceneCategory(t *testing.T) {
	t.Parallel()

	for _, marketplace := range []string{"amazon", "etsy"} {
		for _, category := range []string{"unknown", "bad/category", "\x00", "\x1f"} {
			profile, err := ResolveProductImageProfile(ProfileInput{
				Marketplace: marketplace, Country: "us", ProductType: "running shoe", SceneCategory: category,
			})
			require.ErrorIs(t, err, ErrInvalidProfileInput, "%s/%q", marketplace, category)
			require.Equal(t, ProductImageProfile{}, profile)
		}
	}
}

func TestResolveProductImageProfileSupportedCategoriesComposeWithProductImage(t *testing.T) {
	t.Parallel()

	for _, category := range []string{"shoes", "jewelry", "bags"} {
		profile, err := ResolveProductImageProfile(ProfileInput{
			Marketplace: "amazon", Country: "us", ProductType: "desk lamp", SceneCategory: category,
		})
		require.NoError(t, err)
		require.Equal(t, "platform_category", profile.SceneDefaultsSource)
		require.Equal(t, category, profile.SceneDefaults.SceneCategory)

		options, err := productimage.MergeSceneOptions(nil, &profile.SceneDefaults)
		require.NoError(t, err)
		require.NotNil(t, options)
		plan, err := productimage.BuildScenePlan(productimage.ScenePlanRequest{
			ProfileName: "local_canvas_default",
			Product: productimage.ProductContext{
				ProductKey: "product-1", Title: "Desk Lamp", ProductType: "desk lamp",
			},
			Options:  *options,
			Geometry: productimage.SceneLayoutInput{CanvasSize: 1600, SubjectWidth: 720, SubjectHeight: 720},
		})
		require.NoError(t, err)
		require.Equal(t, category, plan.Options.SceneCategory)
	}
}

func TestResolveProductImageProfileUnknownMarketplaceKeepsNoDefaultsForSupportedCategory(t *testing.T) {
	t.Parallel()

	profile, err := ResolveProductImageProfile(ProfileInput{
		Marketplace: "etsy", Country: "us", ProductType: "desk lamp", SceneCategory: "shoes",
	})
	require.NoError(t, err)
	require.Equal(t, "default", profile.Family)
	require.Equal(t, "none", profile.SceneDefaultsSource)
	require.Equal(t, productimage.SceneOptions{}, profile.SceneDefaults)
}

func TestResolveProductImageProfilePreservesFourMarketplaceSceneDefaults(t *testing.T) {
	t.Parallel()

	tests := map[string]productimage.SceneOptions{
		"amazon":  {SceneStyle: "studio", BackgroundTone: "bright", Composition: "centered", PropsLevel: "none", AudienceHint: "premium"},
		"shein":   {SceneStyle: "lifestyle", BackgroundTone: "warm", Composition: "close_up", PropsLevel: "light", AudienceHint: "youthful"},
		"temu":    {SceneStyle: "lifestyle", BackgroundTone: "bright", Composition: "multi_angle", PropsLevel: "moderate", AudienceHint: "sporty"},
		"walmart": {SceneStyle: "lifestyle", BackgroundTone: "neutral", Composition: "centered", PropsLevel: "light", AudienceHint: "homey"},
	}
	for marketplace, want := range tests {
		profile, err := ResolveProductImageProfile(ProfileInput{Marketplace: marketplace, Country: "us", ProductType: "desk lamp"})
		require.NoError(t, err)
		require.Equal(t, "platform", profile.SceneDefaultsSource)
		require.Equal(t, want, profile.SceneDefaults)
	}
}

func TestResolveProductImageProfileReturnsIndependentSceneOptions(t *testing.T) {
	t.Parallel()

	first, err := ResolveProductImageProfile(ProfileInput{Marketplace: "shein", Country: "us", ProductType: "bag"})
	require.NoError(t, err)
	first.SceneDefaults.StyleReferenceIDs = append(first.SceneDefaults.StyleReferenceIDs, "mutated")

	second, err := ResolveProductImageProfile(ProfileInput{Marketplace: "shein", Country: "us", ProductType: "bag"})
	require.NoError(t, err)
	require.Empty(t, second.SceneDefaults.StyleReferenceIDs)
}

func TestResolveProductImageProfileReturnsFiniteBoundedThresholds(t *testing.T) {
	t.Parallel()

	productTypes := []string{"shoe", "shirt", "handbag", "blanket", "speaker", "watch", "perfume", "desk lamp"}
	for _, productType := range productTypes {
		profile, err := ResolveProductImageProfile(ProfileInput{Marketplace: "amazon", Country: "us", ProductType: productType})
		require.NoError(t, err)
		for _, threshold := range []float64{profile.MainReviewThreshold, profile.WhiteBackgroundReviewThreshold, profile.WhiteCanvasPenalty} {
			require.False(t, math.IsNaN(threshold))
			require.False(t, math.IsInf(threshold, 0))
			require.GreaterOrEqual(t, threshold, 0.0)
			require.LessOrEqual(t, threshold, 1.0)
		}
	}
}

func TestResolveProductImageProfileIsConcurrentAndDeterministic(t *testing.T) {
	t.Parallel()

	const workers = 32
	want := ProductImageProfile{
		Family: "footwear", Marketplace: "amazon", Country: "us",
		MainReviewThreshold: 0.61, WhiteBackgroundReviewThreshold: 0.68, WhiteCanvasPenalty: 0.04,
		SceneDefaults:       productimage.SceneOptions{SceneCategory: "shoes", SceneStyle: "studio", BackgroundTone: "bright", Composition: "centered", PropsLevel: "none", AudienceHint: "premium"},
		SceneDefaultsSource: "platform_category",
	}
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 100 {
				got, err := ResolveProductImageProfile(ProfileInput{Marketplace: " AMAZON ", Country: " US ", ProductType: "shoe ring"})
				if err != nil {
					errors <- err
					return
				}
				if !productImageProfilesEqual(got, want) {
					errors <- errProfileMismatch
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

var errProfileMismatch = errors.New("profile mismatch")

func productImageProfilesEqual(left, right ProductImageProfile) bool {
	return left.Family == right.Family &&
		left.Marketplace == right.Marketplace &&
		left.Country == right.Country &&
		left.MainReviewThreshold == right.MainReviewThreshold &&
		left.WhiteBackgroundReviewThreshold == right.WhiteBackgroundReviewThreshold &&
		left.WhiteCanvasPenalty == right.WhiteCanvasPenalty &&
		left.SceneDefaultsSource == right.SceneDefaultsSource &&
		reflect.DeepEqual(left.SceneDefaults, right.SceneDefaults)
}
