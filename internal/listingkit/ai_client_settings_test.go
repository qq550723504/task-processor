package listingkit

import (
	"context"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestUpdateAIClientSettingsPreservesExistingKeyWhenRequestKeyBlank(t *testing.T) {
	store := &fakeAIClientCredentialStore{
		credential: &openaiclient.AIClientCredential{
			TenantID:      "tenant-a",
			ClientName:    "default",
			APIKey:        "existing-key",
			BaseURL:       "https://old.example.test/v1",
			Model:         "old-model",
			TimeoutSecond: 30,
			Enabled:       true,
			UpdatedAt:     time.Now(),
		},
	}
	svc := &service{aiCredentialStore: store}
	ctx := openaiclient.WithTenantID(context.Background(), "tenant-a")

	_, err := svc.UpdateAIClientSettings(ctx, &AIClientSettings{
		Scope:         "tenant",
		ClientName:    "default",
		BaseURL:       "https://new.example.test/v1",
		Model:         "new-model",
		TimeoutSecond: 45,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("UpdateAIClientSettings returned error: %v", err)
	}

	if store.saved == nil {
		t.Fatal("expected credential to be saved")
	}
	if store.saved.APIKey != "existing-key" {
		t.Fatalf("saved APIKey = %q, want existing-key", store.saved.APIKey)
	}
	if store.saved.BaseURL != "https://new.example.test/v1" || store.saved.Model != "new-model" || store.saved.TimeoutSecond != 45 {
		t.Fatalf("saved credential = %#v", store.saved)
	}
}

type fakeAIClientCredentialStore struct {
	credential *openaiclient.AIClientCredential
	saved      *openaiclient.AIClientCredential
}

func (f *fakeAIClientCredentialStore) SaveCredential(ctx context.Context, credential openaiclient.AIClientCredential) error {
	saved := credential
	saved.UpdatedAt = time.Now()
	f.saved = &saved
	f.credential = &saved
	return nil
}

func (f *fakeAIClientCredentialStore) GetCredential(ctx context.Context, tenantID, userID, clientName string) (*openaiclient.AIClientCredential, error) {
	if f.credential == nil {
		return nil, nil
	}
	credential := *f.credential
	return &credential, nil
}
