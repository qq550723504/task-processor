package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestRepositoryBuilderDoesNotOwnRuntimeSchemaMigration(t *testing.T) {
	source, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`"task-processor/internal/app/schema/productlisting"`,
		"ProductListingAPIRuntimeAutoMigrateEnabled(",
		"productlisting.Migrate(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("productenrich repository builder must be pure construction; found %s", forbidden)
		}
	}
}
