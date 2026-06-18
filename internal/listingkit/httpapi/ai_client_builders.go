package httpapi

import (
	"fmt"
	"time"

	"task-processor/internal/core/config"
	nanobanana "task-processor/internal/infra/clients/nanobanana"
	openaiclient "task-processor/internal/infra/clients/openai"
)

func buildStrictListingKitChatClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) openaiclient.ChatCompleter {
	return &strictListingKitChatClient{
		clientName: clientName,
		resolver:   resolver,
		fallback:   buildListingKitClientFallback(cfg, clientName),
		cache:      make(map[string]*openaiclient.Client),
	}
}

func buildStrictListingKitImageClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) openaiclient.ImageGenerator {
	return &strictListingKitConfiguredImageClient{
		clientName: clientName,
		resolver:   resolver,
		fallback:   buildListingKitClientFallback(cfg, clientName),
		cache:      make(map[string]openaiclient.ImageGenerator),
		build: func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error) {
			client := openaiclient.NewClient(cfg)
			if client == nil {
				return nil, fmt.Errorf("failed to initialize openai image client")
			}
			return client, nil
		},
	}
}

func buildStrictListingKitNanobananaImageClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) openaiclient.ImageGenerator {
	return &strictListingKitConfiguredImageClient{
		clientName: clientName,
		resolver:   resolver,
		fallback:   buildListingKitClientFallback(cfg, clientName),
		cache:      make(map[string]openaiclient.ImageGenerator),
		build: func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error) {
			return nanobanana.NewClient(nanobanana.Config{
				APIKey:       cfg.APIKey,
				Model:        cfg.Model,
				SubmitURL:    cfg.BaseURL,
				PollInterval: time.Second,
				Timeout:      cfg.Timeout,
			}), nil
		},
	}
}
