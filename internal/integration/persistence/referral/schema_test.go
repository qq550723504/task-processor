//go:build integration

package referral

import (
	"context"
	"testing"
)

func TestSchemaInstallRejectsExistingSchemaWithoutChanges(t *testing.T) {
	owner, _ := ownedDatabase(t)
	if err := Install(context.Background(), owner); err == nil {
		t.Fatal("must not migrate existing schema")
	}
	var count int64
	if err := owner.Raw("SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&count).Error; err != nil || count != 5 {
		t.Fatalf("schema mutated count=%d err=%v", count, err)
	}
}
