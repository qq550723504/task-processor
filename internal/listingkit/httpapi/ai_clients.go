package httpapi

import (
	"fmt"
	"time"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
)

const (
	listingKitImageClientName             = "image"
	listingKitImageClientNameGPTImage2    = "image_gpt_image_2"
	listingKitImageClientNameNanobanana   = "image_nanobanana"
	listingKitImageModelSelectorGPTImage2 = "gpt-image-2"
	listingKitImageModelSelectorNano      = "nanobanana"
	sheinSaleAttributeClientName          = "scorer"
	listingKitStudioImageMinTimeout       = 300 * time.Second
)

func BuildSheinCategoryLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
	return buildStrictListingKitChatClient(cfg, resolver, "default")
}

func BuildSheinSaleAttributeLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
	return buildStrictListingKitChatClient(cfg, resolver, sheinSaleAttributeClientName)
}

func BuildStudioImageGenerator(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {
	return buildListingKitRoutedImageClient(cfg, resolver)
}

func errListingKitAIClientNotConfigured(clientName string) error {
	return fmt.Errorf("listingkit ai client %q is not configured for current tenant/user", normalizeListingKitClientName(clientName))
}
