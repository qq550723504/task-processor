package api

import (
	"context"
	"testing"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func TestWithDependenciesConfiguresSubscriptionState(t *testing.T) {
	t.Parallel()

	repo := listingsubscription.NewMemRepository()
	service, err := listingsubscription.NewService(repo)
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}

	users := []string{"platform-user"}
	roles := []string{"platform-role"}

	h, err := NewHandler(&stubHandlerCoreService{}, WithDependencies(HandlerDependencies{
		Subscription: SubscriptionDependencies{
			Service:            service,
			PlatformAdminUsers: users,
			PlatformAdminRoles: roles,
		},
	}))
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	if h.subscriptionService != service {
		t.Fatal("expected subscription service to be attached")
	}
	if h.subscriptionHandler == nil {
		t.Fatal("expected subscription handler to be initialized")
	}
	if len(h.platformAdminUsers) != 1 || h.platformAdminUsers[0] != "platform-user" {
		t.Fatalf("platform admin users = %#v", h.platformAdminUsers)
	}
	if len(h.platformAdminRoles) != 1 || h.platformAdminRoles[0] != "platform-role" {
		t.Fatalf("platform admin roles = %#v", h.platformAdminRoles)
	}

	users[0] = "mutated-user"
	roles[0] = "mutated-role"
	if h.platformAdminUsers[0] != "platform-user" {
		t.Fatalf("platform admin users should be copied, got %#v", h.platformAdminUsers)
	}
	if h.platformAdminRoles[0] != "platform-role" {
		t.Fatalf("platform admin roles should be copied, got %#v", h.platformAdminRoles)
	}
}

type stubSettingsHandlerService struct {
	gotAIQuery settingsNamespaceQuery
	aiResult   *listingkit.AIClientSettings
}

func (s *stubSettingsHandlerService) GetSheinSettings(context.Context) (*listingkit.SheinSettings, error) {
	return nil, nil
}

func (s *stubSettingsHandlerService) UpdateSheinSettings(context.Context, *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, nil
}

func (s *stubSettingsHandlerService) GetAIClientSettings(_ context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	s.gotAIQuery = settingsNamespaceQuery{Scope: scope, ClientName: clientName}
	if s.aiResult != nil {
		return s.aiResult, nil
	}
	return &listingkit.AIClientSettings{Scope: scope, ClientName: clientName}, nil
}

func (s *stubSettingsHandlerService) UpdateAIClientSettings(context.Context, *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
	return nil, nil
}

func TestWithSettingsHandlerServiceOverridesDefaultSettingsService(t *testing.T) {
	t.Parallel()

	settingsStub := &stubSettingsHandlerService{
		aiResult: &listingkit.AIClientSettings{
			Scope:      "user",
			ClientName: "override",
			Model:      "gpt-test",
		},
	}

	h, err := NewHandler(&stubHandlerCoreService{}, WithSettingsHandlerService(settingsStub))
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	got, err := h.settingsService.Get(context.Background(), "ai", settingsNamespaceQuery{
		Scope:      "user",
		ClientName: "override",
	})
	if err != nil {
		t.Fatalf("settingsService.Get returned error: %v", err)
	}

	if settingsStub.gotAIQuery.Scope != "user" || settingsStub.gotAIQuery.ClientName != "override" {
		t.Fatalf("settings query = %+v", settingsStub.gotAIQuery)
	}
	aiSettings, ok := got.(*listingkit.AIClientSettings)
	if !ok {
		t.Fatalf("settings type = %T, want *listingkit.AIClientSettings", got)
	}
	if aiSettings.Model != "gpt-test" {
		t.Fatalf("ai settings = %+v", aiSettings)
	}
}

func TestNewHandlerAllowsExplicitCoreServicesWithoutBaseService(t *testing.T) {
	t.Parallel()

	core := &stubHandlerCoreService{}
	h, err := NewHandler(nil,
		WithTaskLifecycleService(core),
		WithGenerationTaskService(core),
		WithStudioMediaService(core),
	)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	if h.taskLifecycleService != core {
		t.Fatal("expected explicit task lifecycle service to be attached")
	}
	if h.generationTaskService != core {
		t.Fatal("expected explicit generation task service to be attached")
	}
	if h.studioMediaService != core {
		t.Fatal("expected explicit studio media service to be attached")
	}
}
