package httpapi

import (
	"context"
	"fmt"

	policyassembly "task-processor/internal/app/imageagentpolicy"
	"task-processor/internal/imageagent"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
)

type imageAgentPolicyAvailability struct {
	resolver *imagepolicy.Resolver
}

func loadImageAgentPolicyAvailability() (imageagent.ImagePolicyAvailability, error) {
	resolver, err := policyassembly.LoadEmbeddedResolver()
	if err != nil {
		return nil, fmt.Errorf("load embedded image policy catalog: %w", err)
	}
	return imageAgentPolicyAvailability{resolver: resolver}, nil
}

func (availability imageAgentPolicyAvailability) ValidateAvailable(_ context.Context, marketplace string, policy imageagent.ImagePolicyContext) error {
	if availability.resolver == nil {
		return imagepolicy.ErrInvalidProfileInput
	}
	_, err := availability.resolver.Resolve(imagepolicy.ProfileInput{
		Marketplace: marketplace, Country: policy.Country,
		Family: policy.Family, SceneCategory: policy.SceneCategory,
	})
	return err
}
