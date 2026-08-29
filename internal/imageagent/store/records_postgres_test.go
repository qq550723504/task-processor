package store

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm/schema"
)

func TestImageAgentV2RecordsUseOwnerScopedPrimaryKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		model      any
		table      string
		primaryKey []string
	}{
		{name: "run", model: &runRecord{}, table: "image_agent_v2_runs", primaryKey: []string{"TenantID", "UserID", "ID"}},
		{name: "plan", model: &planRecord{}, table: "image_agent_v2_plans", primaryKey: []string{"TenantID", "OwnerUserID", "RunID", "Revision"}},
		{name: "slot", model: &slotRecord{}, table: "image_agent_v2_slots", primaryKey: []string{"TenantID", "OwnerUserID", "RunID", "PlanRevision", "ID"}},
		{name: "attempt", model: &attemptRecord{}, table: "image_agent_v2_attempts", primaryKey: []string{"TenantID", "OwnerUserID", "RunID", "PlanRevision", "SlotID", "Attempt"}},
		{name: "event", model: &eventRecord{}, table: "image_agent_v2_events", primaryKey: []string{"TenantID", "OwnerUserID", "RunID", "Cursor"}},
		{name: "catalog asset", model: &assetCatalogRecord{}, table: "image_agent_v2_asset_catalog", primaryKey: []string{"TenantID", "OwnerUserID", "RunID", "ID"}},
		{name: "catalog manifest", model: &assetCatalogManifestRecord{}, table: "image_agent_v2_asset_catalog_manifests", primaryKey: []string{"TenantID", "OwnerUserID", "RunID"}},
		{name: "projection", model: &projectionRecord{}, table: "image_agent_v2_projection_snapshots", primaryKey: []string{"TenantID", "OwnerUserID", "RunID"}},
		{name: "projection commit", model: &projectionCommitRecord{}, table: "image_agent_v2_projection_commits", primaryKey: []string{"TenantID", "OwnerUserID", "RunID", "CommitID"}},
		{name: "slot external effect", model: &slotExternalEffectRecord{}, table: "image_agent_v2_slot_external_effects", primaryKey: []string{"TenantID", "OwnerUserID", "RunID", "PlanRevision", "SlotID", "Attempt"}},
		{name: "slot external effect v3", model: &slotExternalEffectV3Record{}, table: "image_agent_v3_slot_external_effects", primaryKey: []string{"TenantID", "OwnerUserID", "RunID", "PlanRevision", "SlotID", "Attempt"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := schema.Parse(test.model, &sync.Map{}, schema.NamingStrategy{})
			require.NoError(t, err)
			require.Equal(t, test.table, parsed.Table)

			primaryKeys := make([]string, 0, len(parsed.PrimaryFields))
			for _, field := range parsed.PrimaryFields {
				primaryKeys = append(primaryKeys, field.Name)
			}
			require.Equal(t, test.primaryKey, primaryKeys)

			ownerFieldName := "OwnerUserID"
			if reflect.TypeOf(test.model).Elem() == reflect.TypeOf(runRecord{}) {
				ownerFieldName = "UserID"
			}
			require.Equal(t, "owner_user_id", parsed.FieldsByName[ownerFieldName].DBName)
		})
	}
}

func TestImageAgentBinaryRecordsUsePostgresBytea(t *testing.T) {
	t.Parallel()

	dialector := postgres.New(postgres.Config{}).(interface {
		DataTypeOf(*schema.Field) string
	})
	for _, test := range []struct {
		name   string
		model  any
		fields []string
	}{
		{name: "run", model: &runRecord{}, fields: []string{"BudgetJSON", "UsageJSON", "BlockJSON"}},
		{name: "plan", model: &planRecord{}, fields: []string{"SourceAssetIDs", "StyleReferenceIDs"}},
		{name: "slot", model: &slotRecord{}, fields: []string{"SourceAssetIDs", "StyleReferenceIDs", "CandidateAssetIDs"}},
		{name: "event", model: &eventRecord{}, fields: []string{"Payload"}},
		{name: "slot external effect", model: &slotExternalEffectRecord{}, fields: []string{"GeneratedJSON", "PublishedJSON"}},
		{name: "slot external effect v3", model: &slotExternalEffectV3Record{}, fields: []string{"StagingManifestJSON", "FinalManifestJSON", "PublishedJSON"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := schema.Parse(test.model, &sync.Map{}, schema.NamingStrategy{})
			require.NoError(t, err)
			for _, name := range test.fields {
				field := parsed.FieldsByName[name]
				require.NotNil(t, field)
				require.Equal(t, "bytea", dialector.DataTypeOf(field), "%s.%s", test.name, name)
			}
		})
	}
}
