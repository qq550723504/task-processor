package httpapi

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
)

func TestBuildBootstrapCompositionFailureClosesEveryOwnedResourceInReverseConstructionOrder(t *testing.T) {
	order := make([]string, 0, 4)
	deps := newBootstrapOwnershipTestDeps(&order)
	wantErr := errors.New("composition failed")

	_, err := buildBootstrapWithDependencies(logrus.New(), Options{}, bootstrapBuildDependencies{
		buildRuntimeDeps: func(*logrus.Logger, string) (*runtimeDeps, error) {
			return deps, nil
		},
		buildComposition: func(*logrus.Logger, *runtimeDeps) (httpFeatureComposition, error) {
			deps.addClosers(recordBootstrapClose(&order, "composition-resource"))
			return httpFeatureComposition{}, wantErr
		},
		buildRuntimeBundle: func(httpFeatureComposition, *config.Config) (runtimeBundle, error) {
			t.Fatal("runtime bundle builder called after composition failure")
			return runtimeBundle{}, nil
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildBootstrapWithDependencies() error = %v, want %v", err, wantErr)
	}
	assertBootstrapCloseOrder(t, order, []string{
		"composition-resource",
		"runtime-after-feature",
		"feature-flags",
		"runtime-before-feature",
	})
}

func TestBuildBootstrapRuntimeBundleFailureClosesEveryOwnedResourceInReverseConstructionOrder(t *testing.T) {
	order := make([]string, 0, 4)
	deps := newBootstrapOwnershipTestDeps(&order)
	wantErr := errors.New("runtime bundle failed")

	_, err := buildBootstrapWithDependencies(logrus.New(), Options{}, bootstrapBuildDependencies{
		buildRuntimeDeps: func(*logrus.Logger, string) (*runtimeDeps, error) {
			return deps, nil
		},
		buildComposition: func(*logrus.Logger, *runtimeDeps) (httpFeatureComposition, error) {
			deps.addClosers(recordBootstrapClose(&order, "composition-resource"))
			return httpFeatureComposition{}, nil
		},
		buildRuntimeBundle: func(httpFeatureComposition, *config.Config) (runtimeBundle, error) {
			return runtimeBundle{}, wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildBootstrapWithDependencies() error = %v, want %v", err, wantErr)
	}
	assertBootstrapCloseOrder(t, order, []string{
		"composition-resource",
		"runtime-after-feature",
		"feature-flags",
		"runtime-before-feature",
	})
}

func TestBuildBootstrapSuccessfulShutdownClosesOrdinaryResourcesForwardThenFeatureFlagsLast(t *testing.T) {
	order := make([]string, 0, 4)
	deps := newBootstrapOwnershipTestDeps(&order)

	bootstrap, err := buildBootstrapWithDependencies(logrus.New(), Options{}, bootstrapBuildDependencies{
		buildRuntimeDeps: func(*logrus.Logger, string) (*runtimeDeps, error) {
			return deps, nil
		},
		buildComposition: func(*logrus.Logger, *runtimeDeps) (httpFeatureComposition, error) {
			deps.addClosers(recordBootstrapClose(&order, "composition-resource"))
			return httpFeatureComposition{}, nil
		},
		buildRuntimeBundle: func(httpFeatureComposition, *config.Config) (runtimeBundle, error) {
			return runtimeBundle{}, nil
		},
	})
	if err != nil {
		t.Fatalf("buildBootstrapWithDependencies() error = %v", err)
	}
	if got := len(deps.shared.closers); got != 3 {
		t.Fatalf("intermediate ordinary closers = %d, want 3 without feature shutdown", got)
	}
	if got := len(bootstrap.closers); got != 4 {
		t.Fatalf("bootstrap closers = %d, want 3 ordinary closers and feature shutdown", got)
	}

	closeResources(logrus.New(), bootstrap.closers)
	assertBootstrapCloseOrder(t, order, []string{
		"runtime-before-feature",
		"runtime-after-feature",
		"composition-resource",
		"feature-flags",
	})
}

func newBootstrapOwnershipTestDeps(order *[]string) *runtimeDeps {
	beforeFeature := recordBootstrapClose(order, "runtime-before-feature")
	featureFlags := recordBootstrapClose(order, "feature-flags")
	afterFeature := recordBootstrapClose(order, "runtime-after-feature")
	return &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg:     &config.Config{},
			closers: []func() error{beforeFeature, afterFeature},
		},
		features:            &featureRuntimeState{},
		constructionClosers: []func() error{beforeFeature, featureFlags, afterFeature},
		featureFlagsCloser:  featureFlags,
	}
}

func recordBootstrapClose(order *[]string, name string) func() error {
	return func() error {
		*order = append(*order, name)
		return nil
	}
}

func assertBootstrapCloseOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("close order = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("close order = %#v, want %#v", got, want)
		}
	}
}
