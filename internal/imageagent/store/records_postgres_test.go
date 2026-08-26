package store

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm/schema"
)

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
