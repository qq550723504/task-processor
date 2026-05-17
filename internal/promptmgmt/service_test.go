package promptmgmt

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/prompt"
)

type stubPromptRegistry struct {
	contentByKey map[string]string
}

func (s stubPromptRegistry) Get(key string, _ string) string {
	return s.contentByKey[key]
}

func (s stubPromptRegistry) Render(string, map[string]any, string) (string, error) {
	return "", nil
}

func (s stubPromptRegistry) GetTenant(string, string) (string, error) {
	return "", nil
}

func (s stubPromptRegistry) RenderTenant(string, string, map[string]any) (string, error) {
	return "", nil
}

func (s stubPromptRegistry) Keys() []string {
	keys := make([]string, 0, len(s.contentByKey))
	for key := range s.contentByKey {
		keys = append(keys, key)
	}
	return keys
}

func TestServiceRequiresStore(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.ListTenantTemplates(context.Background(), "tenant-a"); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("ListTenantTemplates error = %v, want ErrServiceUnavailable", err)
	}
	if _, err := svc.UpsertTenantTemplate(context.Background(), UpsertTemplateInput{}); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("UpsertTenantTemplate error = %v, want ErrServiceUnavailable", err)
	}
	if _, err := svc.SetTenantTemplateStatus(context.Background(), "tenant-a", "key", true); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("SetTenantTemplateStatus error = %v, want ErrServiceUnavailable", err)
	}
}

func TestServiceListsTemplateCatalog(t *testing.T) {
	originalRegistry := prompt.GlobalRegistry
	prompt.GlobalRegistry = stubPromptRegistry{
		contentByKey: map[string]string{
			prompt.KSheinContentOptimizerOptimizeTitleDescriptionUser: "Title: {{.title}}\nDescription: {{.description}}",
		},
	}
	defer func() {
		prompt.GlobalRegistry = originalRegistry
	}()

	svc := NewService(nil)
	items := svc.ListTemplateCatalog()
	if len(items) == 0 {
		t.Fatal("ListTemplateCatalog returned no items")
	}

	var matched *TemplateSchema
	for i := range items {
		if items[i].Key == prompt.KSheinContentOptimizerOptimizeTitleDescriptionUser {
			matched = &items[i]
			break
		}
	}
	if matched == nil {
		t.Fatalf("catalog missing key %s", prompt.KSheinContentOptimizerOptimizeTitleDescriptionUser)
	}
	if !matched.HasDefaultContent {
		t.Fatalf("catalog item %+v should report default content", *matched)
	}
	if matched.Label != "文案优化 / 用户提示词" {
		t.Fatalf("catalog label = %q, want explicit label", matched.Label)
	}
	if len(matched.Variables) != 3 {
		t.Fatalf("variables = %+v, want 3 items", matched.Variables)
	}
	if matched.Variables[0].Description == "" {
		t.Fatalf("variables = %+v, want glossary-backed descriptions", matched.Variables)
	}
}

func TestServiceGetsTemplateSchema(t *testing.T) {
	originalRegistry := prompt.GlobalRegistry
	prompt.GlobalRegistry = stubPromptRegistry{
		contentByKey: map[string]string{},
	}
	defer func() {
		prompt.GlobalRegistry = originalRegistry
	}()

	svc := NewService(nil)
	schema, err := svc.GetTemplateSchema(prompt.KProductImageStudioGenerationPodDesign)
	if err != nil {
		t.Fatalf("GetTemplateSchema error = %v", err)
	}
	if schema.Category != "productimage" || schema.Group != "image" || !schema.SupportsTenantOverride {
		t.Fatalf("schema = %+v", schema)
	}
	if len(schema.SupportedScopes) != 1 || schema.SupportedScopes[0].ID != "tenant" {
		t.Fatalf("schema scopes = %+v, want tenant scope from manifest", schema.SupportedScopes)
	}
	if len(schema.Variables) != 4 {
		t.Fatalf("schema variables = %+v, want 4 items", schema.Variables)
	}
	if schema.Variables[0].Key != "TransparentHint" {
		t.Fatalf("schema variables = %+v, want explicit manifest order", schema.Variables)
	}
	if _, err := svc.GetTemplateSchema("missing.key"); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("GetTemplateSchema missing error = %v, want ErrTemplateNotFound", err)
	}
}

type stubPromptStore struct {
	list   []prompt.TenantPromptTemplate
	get    *prompt.TenantPromptTemplate
	getErr error
	last   prompt.TenantPromptTemplate
	status struct {
		tenantID string
		key      string
		enabled  bool
	}
}

func (s *stubPromptStore) GetEnabled(_ context.Context, tenantID string, key string) (*prompt.TenantPromptTemplate, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.get, nil
}

func (s *stubPromptStore) ListTenant(_ context.Context, _ string) ([]prompt.TenantPromptTemplate, error) {
	return s.list, nil
}

func (s *stubPromptStore) SetEnabled(_ context.Context, tenantID string, key string, enabled bool) error {
	s.status.tenantID = tenantID
	s.status.key = key
	s.status.enabled = enabled
	return nil
}

func (s *stubPromptStore) Upsert(_ context.Context, tmpl prompt.TenantPromptTemplate) error {
	s.last = tmpl
	return nil
}

func TestServiceUpsertNormalizesTemplate(t *testing.T) {
	store := &stubPromptStore{
		get: &prompt.TenantPromptTemplate{
			TenantID: "tenant-a",
			Key:      prompt.KSheinContentOptimizerOptimizeTitleDescriptionSystem,
			Content:  "hello",
			Version:  "v1",
			Enabled:  true,
		},
	}
	svc := NewService(store)
	saved, err := svc.UpsertTenantTemplate(context.Background(), UpsertTemplateInput{
		TenantID: " tenant-a ",
		Key:      " " + prompt.KSheinContentOptimizerOptimizeTitleDescriptionSystem + " ",
		Content:  " hello ",
		Version:  " v1 ",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("UpsertTenantTemplate error = %v", err)
	}
	if store.last.TenantID != "tenant-a" || store.last.Key != prompt.KSheinContentOptimizerOptimizeTitleDescriptionSystem || store.last.Content != "hello" || store.last.Version != "v1" {
		t.Fatalf("normalized template = %+v", store.last)
	}
	if saved.Key != prompt.KSheinContentOptimizerOptimizeTitleDescriptionSystem || saved.TenantID != "tenant-a" {
		t.Fatalf("saved = %+v", saved)
	}
}

func TestServiceUpsertRejectsUnknownTemplateKey(t *testing.T) {
	store := &stubPromptStore{}
	svc := NewService(store)

	_, err := svc.UpsertTenantTemplate(context.Background(), UpsertTemplateInput{
		TenantID: "tenant-a",
		Key:      "unknown.prompt.key",
		Content:  "hello",
		Enabled:  true,
	})
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("UpsertTenantTemplate unknown key error = %v, want ErrTemplateNotFound", err)
	}
	if store.last.Key != "" {
		t.Fatalf("store should not be called for unknown key, got %+v", store.last)
	}
}

func TestServiceSetStatusReturnsNormalizedPayload(t *testing.T) {
	store := &stubPromptStore{}
	svc := NewService(store)
	status, err := svc.SetTenantTemplateStatus(context.Background(), " tenant-a ", " tmpl.key ", false)
	if err != nil {
		t.Fatalf("SetTenantTemplateStatus error = %v", err)
	}
	if store.status.tenantID != "tenant-a" || store.status.key != "tmpl.key" || store.status.enabled {
		t.Fatalf("store status = %+v", store.status)
	}
	if status.TenantID != "tenant-a" || status.Key != "tmpl.key" || status.Enabled {
		t.Fatalf("status = %+v", status)
	}
}
