package httpapi

import (
	"fmt"
	"time"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

const (
	listingKitImageClientName                     = "image"
	listingKitImageClientNameGPTImage2            = listingkit.ImageAIClientNameGPTImage2
	listingKitImageClientNameNanobanana           = "image_nanobanana"
	listingKitImageClientNameBackgroundRemoval    = "image_background_removal"
	listingKitImageModelSelectorGPTImage2         = "gpt-image-2"
	listingKitImageModelSelectorNano              = "nanobanana"
	listingKitImageModelSelectorBackgroundRemoval = "background-removal"
	sheinSaleAttributeClientName                  = "scorer"
	listingKitStudioImageMinTimeout               = 300 * time.Second
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

func BuildStudioBackgroundRemover(cfg *config.Config, resolver openaiclient.ClientConfigResolver) listingkit.StudioBackgroundRemover {
	return adaptListingKitBackgroundRemover(buildStrictListingKitImageClient(cfg, resolver, listingKitImageClientNameBackgroundRemoval))
}

func errListingKitAIClientNotConfigured(clientName string) error {
	return fmt.Errorf("listingkit ai client %q is not configured for current tenant/user", normalizeListingKitClientName(clientName))
}
