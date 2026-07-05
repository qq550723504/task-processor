package httpapi

import (
	"fmt"
	"strings"
	"time"

	"task-processor/internal/core/config"
	geminiimage "task-processor/internal/infra/clients/geminiimage"
	grsai "task-processor/internal/infra/clients/grsai"
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
			switch normalizeImageAPIStyle(cfg) {
			case imageAPIStyleGemini:
				return geminiimage.NewClient(geminiimage.Config{
					APIKey:      cfg.APIKey,
					Model:       cfg.Model,
					BaseURL:     cfg.BaseURL,
					Timeout:     cfg.Timeout,
					MaxAttempts: maxImageClientAttempts(cfg.MaxRetries),
					RetryDelay:  time.Second,
				}), nil
			case imageAPIStyleGRSAI:
				return grsai.NewClient(grsai.Config{
					APIKey:       cfg.APIKey,
					Model:        cfg.Model,
					SubmitURL:    cfg.BaseURL,
					PollInterval: time.Second,
					Timeout:      cfg.Timeout,
					MaxAttempts:  cfg.MaxRetries,
				}), nil
			}
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
			switch normalizeImageAPIStyle(cfg) {
			case imageAPIStyleGemini:
				return geminiimage.NewClient(geminiimage.Config{
					APIKey:      cfg.APIKey,
					Model:       cfg.Model,
					BaseURL:     cfg.BaseURL,
					Timeout:     cfg.Timeout,
					MaxAttempts: maxImageClientAttempts(cfg.MaxRetries),
					RetryDelay:  time.Second,
				}), nil
			default:
				return grsai.NewClient(grsai.Config{
					APIKey:       cfg.APIKey,
					Model:        cfg.Model,
					SubmitURL:    cfg.BaseURL,
					PollInterval: time.Second,
					Timeout:      cfg.Timeout,
					MaxAttempts:  maxImageClientAttempts(cfg.MaxRetries),
				}), nil
			}
		},
	}
}

const (
	imageAPIStyleOpenAI = "openai"
	imageAPIStyleGemini = "gemini"
	imageAPIStyleGRSAI  = "grsai_async"
)

func normalizeImageAPIStyle(cfg *openaiclient.ClientConfig) string {
	if cfg == nil {
		return imageAPIStyleOpenAI
	}
	style := strings.ToLower(strings.TrimSpace(cfg.APIStyle))
	switch style {
	case imageAPIStyleOpenAI:
		return imageAPIStyleOpenAI
	case imageAPIStyleGemini:
		return imageAPIStyleGemini
	case imageAPIStyleGRSAI, "nanobanana":
		return imageAPIStyleGRSAI
	}
	baseURL := strings.ToLower(strings.TrimSpace(cfg.BaseURL))
	if strings.Contains(baseURL, "grsai") {
		return imageAPIStyleGRSAI
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(cfg.Model)), "gemini") {
		return imageAPIStyleGemini
	}
	return imageAPIStyleOpenAI
}

func maxImageClientAttempts(retries int) int {
	if retries <= 0 {
		return 1
	}
	return retries + 1
}
