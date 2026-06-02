package listingadmin

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormGenerationTopicPolicyRepository_CreateAndListGenerationTopicPolicies(t *testing.T) {
	db := openGenerationTopicPolicyTestDB(t)
	if err := AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		t.Fatalf("AutoMigrateGenerationTopicPolicyRepository: %v", err)
	}

	repo := NewGormGenerationTopicPolicyRepository(db)
	createCtx := withRequestIdentity(context.TODO(), "planner", nil)
	readCtx := withRequestIdentity(context.TODO(), "reviewer", nil)

	created, err := repo.CreateGenerationTopicPolicy(createCtx, &GenerationTopicPolicy{
		TenantID: 101,
		Platform: " shein ",
		TopicKey: " children ",
		Remark:   " keep prompt safe ",
		Status:   1,
	})
	if err != nil {
		t.Fatalf("CreateGenerationTopicPolicy: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("created = %+v, want persisted ID", created)
	}
	if created.Platform != "shein" || created.TopicKey != "children" || created.Remark != "keep prompt safe" {
		t.Fatalf("created = %+v, want trimmed fields", created)
	}
	if _, err := repo.CreateGenerationTopicPolicy(createCtx, &GenerationTopicPolicy{
		TenantID: 101,
		Platform: "shein",
		TopicKey: "children",
		Status:   1,
	}); err == nil {
		t.Fatalf("CreateGenerationTopicPolicy duplicate = nil, want unique constraint error")
	}

	page, err := repo.ListGenerationTopicPolicies(readCtx, GenerationTopicPolicyQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListGenerationTopicPolicies: %v", err)
	}
	if page.Total != 1 || page.Page != 1 || page.PageSize != 20 {
		t.Fatalf("page meta = %+v, want total/page/pageSize = 1/1/20", page)
	}
	if len(page.Items) != 1 {
		t.Fatalf("page items = %+v, want exactly one item", page.Items)
	}
	if page.Items[0].Platform != "shein" || page.Items[0].TopicKey != "children" {
		t.Fatalf("page item = %+v, want persisted topic policy", page.Items[0])
	}
}

func TestAutoMigrateGenerationTopicPolicyRepositoryRejectsDuplicateTenantPlatformTopicRows(t *testing.T) {
	db := openGenerationTopicPolicyTestDB(t)
	if err := db.AutoMigrate(&listingGenerationTopicPolicy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingGenerationTopicPolicy{
		{TenantID: 101, Platform: "shein", TopicKey: "children", Status: 1, Deleted: 0},
		{TenantID: 101, Platform: "shein", TopicKey: "children", Status: 0, Deleted: 0},
	} {
		if err := db.Table("listing_generation_topic_policy").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	err := AutoMigrateGenerationTopicPolicyRepository(db)
	if err == nil {
		t.Fatalf("AutoMigrateGenerationTopicPolicyRepository: nil error, want duplicate precheck failure")
	}
	if !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), "tenant_id") || !strings.Contains(err.Error(), "topic_key") {
		t.Fatalf("migration error = %q, want descriptive duplicate constraint failure", err)
	}
}

func openGenerationTopicPolicyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}
