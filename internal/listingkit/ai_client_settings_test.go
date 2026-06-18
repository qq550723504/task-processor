package listingkit

import (
	"context"
	"testing"
	"time"
)

func TestUpdateAIClientSettingsPreservesExistingKeyWhenRequestKeyBlank(t *testing.T) {
	store := &fakeAIClientCredentialStore{
		credentials: map[string]*AIClientCredential{
			"tenant-a||default": {
				TenantID:      "tenant-a",
				ClientName:    "default",
				APIKey:        "existing-key",
				BaseURL:       "https://old.example.test/v1",
				Model:         "old-model",
				TimeoutSecond: 30,
				Enabled:       true,
				UpdatedAt:     time.Now(),
			},
		},
	}
	svc := &service{adminDeps: adminDependencies{aiCredentialStore: store}}
	ctx := WithRequestIdentity(context.Background(), RequestIdentity{TenantID: "tenant-a"})

	_, err := svc.UpdateAIClientSettings(ctx, &AIClientSettings{
		Scope:      "tenant",
		ClientName: "default",
		BaseURL:    "https://new.example.test/v1",
		Model:      "new-model",
		Enabled:    true,
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
	if store.saved.BaseURL != "https://new.example.test/v1" || store.saved.Model != "new-model" || store.saved.TimeoutSecond != 0 {
		t.Fatalf("saved credential = %#v", store.saved)
	}
}

type fakeAIClientCredentialStore struct {
	credentials map[string]*AIClientCredential
	saved       *AIClientCredential
}

func (f *fakeAIClientCredentialStore) SaveCredential(ctx context.Context, credential AIClientCredential) error {
	saved := credential
	saved.UpdatedAt = time.Now()
	f.saved = &saved
	if f.credentials == nil {
		f.credentials = make(map[string]*AIClientCredential)
	}
	f.credentials[aiCredentialKey(saved.TenantID, saved.UserID, saved.ClientName)] = &saved
	return nil
}

func (f *fakeAIClientCredentialStore) GetCredential(ctx context.Context, tenantID, userID, clientName string) (*AIClientCredential, error) {
	if f.credentials == nil {
		return nil, nil
	}
	credential := f.credentials[aiCredentialKey(tenantID, userID, clientName)]
	if credential == nil {
		return nil, nil
	}
	copied := *credential
	return &copied, nil
}

func aiCredentialKey(tenantID, userID, clientName string) string {
	return tenantID + "|" + userID + "|" + clientName
}

func TestGetAIClientSettingsReportsResolvedScope(t *testing.T) {
	now := time.Now()
	store := &fakeAIClientCredentialStore{
		credentials: map[string]*AIClientCredential{
			aiCredentialKey("tenant-a", "", "default"): {
				TenantID:      "tenant-a",
				UserID:        "",
				ClientName:    "default",
				APIKey:        "tenant-key",
				BaseURL:       "https://tenant.example.test/v1",
				Model:         "tenant-model",
				TimeoutSecond: 40,
				Enabled:       true,
				UpdatedAt:     now,
			},
			aiCredentialKey("tenant-a", "user-a", "default"): {
				TenantID:      "tenant-a",
				UserID:        "user-a",
				ClientName:    "default",
				APIKey:        "user-key",
				BaseURL:       "https://user.example.test/v1",
				Model:         "user-model",
				TimeoutSecond: 50,
				Enabled:       true,
				UpdatedAt:     now,
			},
		},
	}
	svc := &service{adminDeps: adminDependencies{aiCredentialStore: store}}
	ctx := WithRequestIdentity(context.Background(), RequestIdentity{
		TenantID: "tenant-a",
		UserID:   "user-a",
	})

	userScoped, err := svc.GetAIClientSettings(ctx, "user", "default")
	if err != nil {
		t.Fatalf("GetAIClientSettings(user) returned error: %v", err)
	}
	if userScoped.ResolvedScope != "user" {
		t.Fatalf("user scoped resolved_scope = %q, want user", userScoped.ResolvedScope)
	}
	if userScoped.BaseURL != "https://user.example.test/v1" {
		t.Fatalf("user scoped base_url = %q", userScoped.BaseURL)
	}

	tenantScoped, err := svc.GetAIClientSettings(ctx, "tenant", "default")
	if err != nil {
		t.Fatalf("GetAIClientSettings(tenant) returned error: %v", err)
	}
	if tenantScoped.ResolvedScope != "tenant" {
		t.Fatalf("tenant scoped resolved_scope = %q, want tenant", tenantScoped.ResolvedScope)
	}
	if tenantScoped.BaseURL != "https://tenant.example.test/v1" {
		t.Fatalf("tenant scoped base_url = %q", tenantScoped.BaseURL)
	}

	missing, err := svc.GetAIClientSettings(ctx, "tenant", "scorer")
	if err != nil {
		t.Fatalf("GetAIClientSettings(missing) returned error: %v", err)
	}
	if missing.ResolvedScope != "" {
		t.Fatalf("missing resolved_scope = %q, want empty", missing.ResolvedScope)
	}
	if missing.APIKeySet {
		t.Fatal("missing config should not report api key set")
	}
}

func TestUpdateAIClientSettingsIgnoresRequestedTimeout(t *testing.T) {
	store := &fakeAIClientCredentialStore{}
	svc := &service{adminDeps: adminDependencies{aiCredentialStore: store}}
	ctx := WithRequestIdentity(context.Background(), RequestIdentity{TenantID: "tenant-a"})

	_, err := svc.UpdateAIClientSettings(ctx, &AIClientSettings{
		Scope:      "tenant",
		ClientName: "image_nanobanana",
		APIKey:     "key",
		BaseURL:    "https://example.test/v1",
		Model:      "nano-banana-fast",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("UpdateAIClientSettings returned error: %v", err)
	}
	if store.saved == nil {
		t.Fatal("expected credential to be saved")
	}
	if store.saved.TimeoutSecond != 0 {
		t.Fatalf("saved timeout = %d, want 0", store.saved.TimeoutSecond)
	}
}

func TestGetAIClientSettingsDoesNotExposeStoredTimeout(t *testing.T) {
	now := time.Now()
	store := &fakeAIClientCredentialStore{
		credentials: map[string]*AIClientCredential{
			aiCredentialKey("tenant-a", "", "image_gpt_image_2"): {
				TenantID:      "tenant-a",
				ClientName:    "image_gpt_image_2",
				APIKey:        "tenant-key",
				BaseURL:       "https://tenant.example.test/v1",
				Model:         "gpt-image-2",
				TimeoutSecond: 60,
				Enabled:       true,
				UpdatedAt:     now,
			},
		},
	}
	svc := &service{adminDeps: adminDependencies{aiCredentialStore: store}}
	ctx := WithRequestIdentity(context.Background(), RequestIdentity{TenantID: "tenant-a"})

	settings, err := svc.GetAIClientSettings(ctx, "tenant", "image_gpt_image_2")
	if err != nil {
		t.Fatalf("GetAIClientSettings returned error: %v", err)
	}
	if settings.ClientName != "image_gpt_image_2" {
		t.Fatalf("client name = %q, want image_gpt_image_2", settings.ClientName)
	}
}
