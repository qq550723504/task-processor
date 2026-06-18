package httpapi

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

type listingKitAICredentialStore struct {
	store aiCredentialStore
}

func adaptListingKitAICredentialStore(store aiCredentialStore) listingkit.AIClientCredentialStore {
	if store == nil {
		return nil
	}
	return listingKitAICredentialStore{store: store}
}

func (s listingKitAICredentialStore) SaveCredential(ctx context.Context, credential listingkit.AIClientCredential) error {
	return s.store.SaveCredential(ctx, openaiclient.AIClientCredential{
		TenantID:      credential.TenantID,
		UserID:        credential.UserID,
		ClientName:    credential.ClientName,
		APIKey:        credential.APIKey,
		BaseURL:       credential.BaseURL,
		Model:         credential.Model,
		TimeoutSecond: credential.TimeoutSecond,
		Enabled:       credential.Enabled,
		UpdatedAt:     credential.UpdatedAt,
	})
}

func (s listingKitAICredentialStore) GetCredential(ctx context.Context, tenantID, userID, clientName string) (*listingkit.AIClientCredential, error) {
	credential, err := s.store.GetCredential(ctx, tenantID, userID, clientName)
	if err != nil || credential == nil {
		return nil, err
	}
	return &listingkit.AIClientCredential{
		TenantID:      credential.TenantID,
		UserID:        credential.UserID,
		ClientName:    credential.ClientName,
		APIKey:        credential.APIKey,
		BaseURL:       credential.BaseURL,
		Model:         credential.Model,
		TimeoutSecond: credential.TimeoutSecond,
		Enabled:       credential.Enabled,
		UpdatedAt:     credential.UpdatedAt,
	}, nil
}
