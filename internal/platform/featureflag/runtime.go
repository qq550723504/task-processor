package featureflag

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/isolated"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
)

type Config struct {
	Flags map[string]bool
}

type Runtime struct {
	api    *openfeature.EvaluationAPI
	client *openfeature.Client
}

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	flags := make(map[string]memprovider.InMemoryFlag, len(cfg.Flags))
	for key, value := range cfg.Flags {
		flags[key] = memprovider.InMemoryFlag{
			Key:            key,
			State:          memprovider.Enabled,
			DefaultVariant: "configured",
			Variants:       map[string]any{"configured": value},
		}
	}

	api := isolated.NewAPI()
	provider := memprovider.NewInMemoryProvider(flags)
	if err := api.SetProviderAndWait(ctx, provider); err != nil {
		setupErr := fmt.Errorf("set OpenFeature provider: %w", err)
		if shutdownErr := api.Shutdown(context.Background()); shutdownErr != nil {
			return nil, errors.Join(setupErr, fmt.Errorf("shutdown OpenFeature API after setup failure: %w", shutdownErr))
		}
		return nil, setupErr
	}
	return &Runtime{api: api, client: api.NewClient()}, nil
}

func (r *Runtime) Bool(ctx context.Context, key string, defaultValue bool, attributes map[string]any) bool {
	return r.client.Boolean(ctx, key, defaultValue, openfeature.NewEvaluationContext("task-processor", attributes))
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	return r.api.Shutdown(ctx)
}
