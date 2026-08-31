package httpapi

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestShouldAutoMigrateProductListingAPIRuntimeUsesNamedFlagAndDefault(t *testing.T) {
	ctx := context.WithValue(t.Context(), evaluatorContextKey{}, "request-context")
	evaluator := &recordingBoolEvaluator{value: false}

	if shouldAutoMigrateProductListingAPIRuntime(ctx, evaluator) {
		t.Fatal("shouldAutoMigrateProductListingAPIRuntime() = true, want evaluator result false")
	}
	if evaluator.ctx.Value(evaluatorContextKey{}) != "request-context" {
		t.Fatal("evaluator did not receive the caller context")
	}
	if evaluator.key != "product-listing-runtime-auto-migrate" {
		t.Fatalf("evaluator key = %q, want product-listing-runtime-auto-migrate", evaluator.key)
	}
	if !evaluator.defaultValue {
		t.Fatal("evaluator default = false, want backwards-compatible true")
	}
	if evaluator.attributes != nil {
		t.Fatalf("evaluator attributes = %#v, want nil", evaluator.attributes)
	}
}

func TestShouldAutoMigrateProductListingAPIRuntimeDefaultsTrueWithoutEvaluator(t *testing.T) {
	if !shouldAutoMigrateProductListingAPIRuntime(t.Context(), nil) {
		t.Fatal("shouldAutoMigrateProductListingAPIRuntime() = false, want default true")
	}
}

func TestBuildRuntimeDepsConstructsFeatureFlagRuntimeAndRegistersShutdown(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	t.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", "false")
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	deps, err := buildRuntimeDeps(logger, "../../../config/config-test.yaml")
	if err != nil {
		t.Fatalf("buildRuntimeDeps() error = %v", err)
	}
	if deps.shared.featureFlags == nil {
		t.Fatal("buildRuntimeDeps() did not retain the feature flag evaluator")
	}
	if shouldAutoMigrateProductListingAPIRuntime(t.Context(), deps.shared.featureFlags) {
		t.Fatal("feature flag evaluator returned true, want environment override false")
	}
	if len(deps.shared.closers) != 2 {
		t.Fatalf("closers = %d, want existing StoreAPI closer followed by feature runtime shutdown", len(deps.shared.closers))
	}
	if err := deps.shared.closers[1](); err != nil {
		t.Fatalf("feature runtime closer error = %v", err)
	}
	if err := deps.shared.closers[0](); err != nil {
		t.Fatalf("StoreAPI closer error = %v", err)
	}
}

func TestCleanupOwnedRuntimeResourcesClosesInReverseOrderOnlyOnFailure(t *testing.T) {
	order := make([]string, 0, 2)
	closers := []func() error{
		func() error {
			order = append(order, "feature-flags")
			return nil
		},
		nil,
		func() error {
			order = append(order, "database")
			return nil
		},
	}

	cleanupOwnedRuntimeResources(false, closers)
	if len(order) != 2 || order[0] != "database" || order[1] != "feature-flags" {
		t.Fatalf("failure cleanup order = %#v, want database then feature-flags", order)
	}

	cleanupOwnedRuntimeResources(true, closers)
	if len(order) != 2 {
		t.Fatalf("successful construction invoked cleanup: %#v", order)
	}
}

type evaluatorContextKey struct{}

type recordingBoolEvaluator struct {
	value        bool
	ctx          context.Context
	key          string
	defaultValue bool
	attributes   map[string]any
}

func (e *recordingBoolEvaluator) Bool(ctx context.Context, key string, defaultValue bool, attributes map[string]any) bool {
	e.ctx = ctx
	e.key = key
	e.defaultValue = defaultValue
	e.attributes = attributes
	return e.value
}
