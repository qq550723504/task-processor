package prompt

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/listingkit/tenantctx"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type promptRegistryStub struct {
	globalPrompts         map[string]string
	globalRenderedPrompts map[string]string
	tenantPrompts         map[string]string
	tenantRenderedPrompts map[string]string
	tenantErr             error
	tenantRenderErr       error
}

func (s promptRegistryStub) Get(key string, _ string) string {
	return s.globalPrompts[key]
}

func (s promptRegistryStub) Render(key string, _ map[string]any, _ string) (string, error) {
	value, ok := s.globalRenderedPrompts[key]
	if !ok {
		return "", errors.New("global render missing")
	}
	return value, nil
}

func (s promptRegistryStub) GetTenant(_ string, key string) (string, error) {
	if s.tenantErr != nil {
		return "", s.tenantErr
	}
	value, ok := s.tenantPrompts[key]
	if !ok {
		return "", ErrTenantPromptNotFound
	}
	return value, nil
}

func (s promptRegistryStub) RenderTenant(_ string, key string, _ map[string]any) (string, error) {
	if s.tenantRenderErr != nil {
		return "", s.tenantRenderErr
	}
	value, ok := s.tenantRenderedPrompts[key]
	if !ok {
		return "", ErrTenantPromptNotFound
	}
	return value, nil
}

func (s promptRegistryStub) Keys() []string { return nil }

func TestGormTenantPromptStore_UpsertAndGetEnabled(t *testing.T) {
	store := openTestPromptStore(t)

	err := store.Upsert(context.Background(), TenantPromptTemplate{
		TenantID: "tenant-a",
		Key:      "shein.content_optimizer.system",
		Content:  "Hello {{.Name}}",
		Version:  "v1",
		Enabled:  true,
	})
	require.NoError(t, err)

	got, err := store.GetEnabled(context.Background(), "tenant-a", "shein.content_optimizer.system")
	require.NoError(t, err)
	require.Equal(t, "Hello {{.Name}}", got.Content)
	require.Equal(t, "v1", got.Version)
}

func TestGormTenantPromptStore_GetEnabledIgnoresDisabledTemplate(t *testing.T) {
	store := openTestPromptStore(t)

	err := store.Upsert(context.Background(), TenantPromptTemplate{
		TenantID: "tenant-a",
		Key:      "shein.content_optimizer.system",
		Content:  "disabled",
		Enabled:  false,
	})
	require.NoError(t, err)

	_, err = store.GetEnabled(context.Background(), "tenant-a", "shein.content_optimizer.system")
	require.True(t, errors.Is(err, ErrTenantPromptNotFound))
}

func TestGormTenantPromptStore_ListTenantOnlyReturnsRequestedTenant(t *testing.T) {
	store := openTestPromptStore(t)
	require.NoError(t, store.Upsert(context.Background(), TenantPromptTemplate{
		TenantID: "tenant-a",
		Key:      "b.key",
		Content:  "B",
		Enabled:  true,
	}))
	require.NoError(t, store.Upsert(context.Background(), TenantPromptTemplate{
		TenantID: "tenant-a",
		Key:      "a.key",
		Content:  "A",
		Enabled:  true,
	}))
	require.NoError(t, store.Upsert(context.Background(), TenantPromptTemplate{
		TenantID: "tenant-b",
		Key:      "c.key",
		Content:  "C",
		Enabled:  true,
	}))

	got, err := store.ListTenant(context.Background(), "tenant-a")

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "a.key", got[0].Key)
	require.Equal(t, "b.key", got[1].Key)
}

func TestGormTenantPromptStore_SetEnabledDisablesTemplate(t *testing.T) {
	store := openTestPromptStore(t)
	require.NoError(t, store.Upsert(context.Background(), TenantPromptTemplate{
		TenantID: "tenant-a",
		Key:      "tmpl.key",
		Content:  "content",
		Enabled:  true,
	}))

	require.NoError(t, store.SetEnabled(context.Background(), "tenant-a", "tmpl.key", false))

	_, err := store.GetEnabled(context.Background(), "tenant-a", "tmpl.key")
	require.True(t, errors.Is(err, ErrTenantPromptNotFound))
}

func TestRegistry_RenderTenantFromContextUsesDatabasePromptBeforeFilePrompt(t *testing.T) {
	store := openTestPromptStore(t)
	require.NoError(t, store.Upsert(context.Background(), TenantPromptTemplate{
		TenantID: "tenant-a",
		Key:      "tmpl.key",
		Content:  "db {{.Name}}",
		Enabled:  true,
	}))

	r := newTestRegistry()
	r.SetTenantPromptStore(store)
	r.tenantCache = map[string]map[string]string{
		"tenant-a": {
			"tmpl.key": "file {{.Name}}",
		},
	}
	ctx := tenantctx.WithTenantID(context.Background(), "tenant-a")

	got, err := r.RenderTenantFromContext(ctx, "tmpl.key", map[string]any{"Name": "Alice"})

	require.NoError(t, err)
	require.Equal(t, "db Alice", got)
}

func TestRegistry_RenderTenantFromContextDoesNotFallBackWhenDatabasePromptMissing(t *testing.T) {
	store := openTestPromptStore(t)
	r := newTestRegistry()
	r.SetTenantPromptStore(store)
	r.tenantCache = map[string]map[string]string{
		"tenant-a": {
			"tmpl.key": "file {{.Name}}",
		},
	}
	ctx := tenantctx.WithTenantID(context.Background(), "tenant-a")

	got, err := r.RenderTenantFromContext(ctx, "tmpl.key", map[string]any{"Name": "Alice"})

	require.Error(t, err)
	require.Empty(t, got)
}

func TestGetTenantFromContextWithGlobalFallbackUsesGlobalPromptWhenTenantMissing(t *testing.T) {
	previous := GlobalRegistry
	GlobalRegistry = promptRegistryStub{
		globalPrompts: map[string]string{"tmpl.key": "global prompt"},
	}
	defer func() { GlobalRegistry = previous }()

	got, err := GetTenantFromContextWithGlobalFallback(context.Background(), "tmpl.key")

	require.NoError(t, err)
	require.Equal(t, "global prompt", got)
}

func TestRenderTenantFromContextWithGlobalFallbackUsesGlobalRenderWhenTenantMissing(t *testing.T) {
	previous := GlobalRegistry
	GlobalRegistry = promptRegistryStub{
		globalRenderedPrompts: map[string]string{"tmpl.key": "global rendered"},
	}
	defer func() { GlobalRegistry = previous }()

	got, err := RenderTenantFromContextWithGlobalFallback(context.Background(), "tmpl.key", map[string]any{"Name": "Alice"})

	require.NoError(t, err)
	require.Equal(t, "global rendered", got)
}

func TestGetTenantFromContextWithGlobalFallbackPreservesNonMissingErrors(t *testing.T) {
	previous := GlobalRegistry
	GlobalRegistry = promptRegistryStub{
		tenantErr: errors.New("db down"),
		globalPrompts: map[string]string{"tmpl.key": "global prompt"},
	}
	defer func() { GlobalRegistry = previous }()

	_, err := GetTenantFromContextWithGlobalFallback(context.Background(), "tmpl.key")

	require.EqualError(t, err, "db down")
}

func openTestPromptStore(t *testing.T) *GormTenantPromptStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TenantPromptTemplate{}))
	return NewGormTenantPromptStore(db)
}
