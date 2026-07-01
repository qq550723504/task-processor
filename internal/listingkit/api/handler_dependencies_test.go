package api

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
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

func TestWithDependenciesConfiguresScheduledTaskConfigHandler(t *testing.T) {
	t.Parallel()

	repo := &listingadmin.GormScheduledTaskConfigRepository{}
	h, err := NewHandler(&stubHandlerCoreService{}, WithDependencies(HandlerDependencies{
		Admin: AdminHandlerDependencies{
			ScheduledTaskConfigRepository: repo,
		},
	}))
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	if h.scheduledTaskConfigRepository != repo {
		t.Fatal("expected scheduled task config repository to be attached")
	}
	if h.scheduledTaskConfigHandler == nil {
		t.Fatal("expected scheduled task config handler to be initialized")
	}
}

type stubSettingsHandlerService struct {
	gotAIQuery settingsNamespaceQuery
	aiResults  map[string]*listingkit.AIClientSettings
	aiResult   *listingkit.AIClientSettings
	shein      *listingkit.SheinSettings
	probes     listingkit.SettingsHealthProbes
}

func (s *stubSettingsHandlerService) GetSheinSettings(context.Context) (*listingkit.SheinSettings, error) {
	return s.shein, nil
}

func (s *stubSettingsHandlerService) UpdateSheinSettings(context.Context, *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, nil
}

func (s *stubSettingsHandlerService) GetAIClientSettings(_ context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	s.gotAIQuery = settingsNamespaceQuery{Scope: scope, ClientName: clientName}
	if s.aiResults != nil {
		return s.aiResults[clientName], nil
	}
	if s.aiResult != nil {
		return s.aiResult, nil
	}
	return &listingkit.AIClientSettings{Scope: scope, ClientName: clientName}, nil
}

func (s *stubSettingsHandlerService) UpdateAIClientSettings(context.Context, *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
	return nil, nil
}

func (s *stubSettingsHandlerService) GetSettingsHealthProbes(context.Context) listingkit.SettingsHealthProbes {
	return s.probes
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

func TestSettingsServiceHealthReadsExistingAIAndSheinSettings(t *testing.T) {
	t.Parallel()

	settingsStub := &stubSettingsHandlerService{
		aiResults: map[string]*listingkit.AIClientSettings{
			"default": {
				ClientName: "default",
				Enabled:    true,
				BaseURL:    "https://api.example.test/v1",
				Model:      "gpt-test",
				APIKeySet:  true,
			},
			"image": {
				ClientName: "image",
				Enabled:    true,
				BaseURL:    "https://api.example.test/v1",
				Model:      "image-test",
				APIKeySet:  true,
			},
		},
		shein: &listingkit.SheinSettings{
			DefaultStoreID:    7,
			Site:              "US",
			DefaultStock:      20,
			DefaultSubmitMode: "save_draft",
		},
		probes: listingkit.SettingsHealthProbes{
			SheinIntegration: listingkit.SettingsHealthProbe{Configured: true},
			SDSLogin:         listingkit.SettingsHealthProbe{Configured: true},
			ObjectStorage:    listingkit.SettingsHealthProbe{Missing: []string{"publisher.s3.bucket 缺失"}},
		},
	}

	health, err := newSettingsService(settingsStub).Health(context.Background())
	if err != nil {
		t.Fatalf("Health returned error: %v", err)
	}

	if health.Status != "blocked" {
		t.Fatalf("health status = %q, want blocked because pricing and object storage are missing", health.Status)
	}
	var hasDefaultAI, hasImageAI, hasSheinProbe, hasSDSProbe, hasPricing, hasObjectStorage bool
	for _, item := range health.Items {
		switch item.Key {
		case "ai.default":
			hasDefaultAI = item.Status == "ready"
		case "ai.image":
			hasImageAI = item.Status == "ready"
		case "shein.integration":
			hasSheinProbe = item.Status == "ready"
		case "sds.session":
			hasSDSProbe = item.Status == "ready"
		case "shein.pricing":
			hasPricing = item.Status == "blocked"
		case "storage.object":
			hasObjectStorage = item.Status == "blocked"
		}
	}
	if !hasDefaultAI || !hasImageAI || !hasSheinProbe || !hasSDSProbe || !hasPricing || !hasObjectStorage {
		t.Fatalf("health items = %#v", health.Items)
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
