package featureflag

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/isolated"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
)

const testFlagKey = "product-listing-runtime-auto-migrate"

func TestRuntimeEvaluatesConfiguredBooleanWithoutGlobalState(t *testing.T) {
	for _, test := range []struct {
		name  string
		value bool
	}{
		{name: "enabled", value: true},
		{name: "disabled", value: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, err := New(t.Context(), Config{Flags: map[string]bool{testFlagKey: test.value}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := runtime.Shutdown(context.Background()); err != nil {
					t.Errorf("Shutdown() error = %v", err)
				}
			})

			if got := runtime.Bool(t.Context(), testFlagKey, !test.value, nil); got != test.value {
				t.Fatalf("Bool() = %t, want configured %t", got, test.value)
			}
			if got := openfeature.NewClient("featureflag-isolation-test").Boolean(t.Context(), testFlagKey, !test.value, openfeature.EvaluationContext{}); got != !test.value {
				t.Fatalf("global OpenFeature Bool() = %t, want caller default %t", got, !test.value)
			}
		})
	}
}

func TestRuntimeReturnsCallerDefaultForUnknownFlag(t *testing.T) {
	runtime, err := New(t.Context(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	if !runtime.Bool(t.Context(), "unknown-flag", true, nil) {
		t.Fatal("Bool() returned false, want caller default true")
	}
	if runtime.Bool(t.Context(), "unknown-flag", false, nil) {
		t.Fatal("Bool() returned true, want caller default false")
	}
}

func TestRuntimeBoolForwardsIsolatedEvaluationAttributes(t *testing.T) {
	evaluator := func(_ memprovider.InMemoryFlag, attributes openfeature.FlattenedContext) (any, openfeature.ProviderResolutionDetail) {
		matches := attributes[openfeature.TargetingKey] == "task-processor" && attributes["tenant"] == "tenant-a"
		attributes["tenant"] = "provider-mutated"
		return matches, openfeature.ProviderResolutionDetail{Reason: openfeature.TargetingMatchReason}
	}
	provider := memprovider.NewInMemoryProvider(map[string]memprovider.InMemoryFlag{
		testFlagKey: {
			Key:              testFlagKey,
			State:            memprovider.Enabled,
			ContextEvaluator: &evaluator,
		},
	})
	runtime := runtimeWithProvider(t, provider)
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	attributes := map[string]any{"tenant": "tenant-a"}
	if !runtime.Bool(t.Context(), testFlagKey, false, attributes) {
		t.Fatal("Bool() returned false, want targeting-key and tenant attribute match")
	}
	if got := attributes["tenant"]; got != "tenant-a" {
		t.Fatalf("caller attributes tenant = %v, want isolated tenant-a", got)
	}
}

func TestRuntimeShutdownDelegatesToIsolatedAPI(t *testing.T) {
	provider := &shutdownTrackingProvider{
		InMemoryProvider: memprovider.NewInMemoryProvider(nil),
		shutdown:         make(chan context.Context, 1),
	}
	runtime := runtimeWithProvider(t, provider)

	ctx := context.WithValue(t.Context(), shutdownContextKey{}, "owned-by-caller")
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	select {
	case got := <-provider.shutdown:
		if got.Value(shutdownContextKey{}) != "owned-by-caller" {
			t.Fatal("Shutdown() did not forward the caller context")
		}
	default:
		t.Fatal("Shutdown() did not invoke provider shutdown")
	}
}

func runtimeWithProvider(t *testing.T, provider openfeature.FeatureProvider) *Runtime {
	t.Helper()
	api := isolated.NewAPI()
	if err := api.SetProviderAndWait(t.Context(), provider); err != nil {
		t.Fatalf("SetProviderAndWait() error = %v", err)
	}
	return &Runtime{api: api, client: api.NewClient()}
}

type shutdownContextKey struct{}

type shutdownTrackingProvider struct {
	memprovider.InMemoryProvider
	shutdown chan context.Context
}

func (p *shutdownTrackingProvider) Init(openfeature.EvaluationContext) error { return nil }

func (p *shutdownTrackingProvider) Shutdown() {}

func (p *shutdownTrackingProvider) InitWithContext(context.Context, openfeature.EvaluationContext) error {
	return nil
}

func (p *shutdownTrackingProvider) ShutdownWithContext(ctx context.Context) error {
	p.shutdown <- ctx
	return nil
}
