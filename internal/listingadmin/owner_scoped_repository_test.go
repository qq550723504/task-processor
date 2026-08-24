package listingadmin

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func newOwnerScopedSQLiteDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate owner-scoped model: %v", err)
	}
	return db
}

func assertOwnerlessWriteRejected(t *testing.T, db *gorm.DB, table string, err error) {
	t.Helper()
	if !errors.Is(err, ErrOwnerUserIDRequired) {
		t.Fatalf("write error = %v, want ErrOwnerUserIDRequired", err)
	}
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s rows: %v", table, err)
	}
	if count != 0 {
		t.Fatalf("%s rows = %d, want 0", table, count)
	}
}

func TestCreateStoreRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingStore{})
	_, err := NewGormStoreRepository(db).CreateStore(context.Background(), &Store{
		TenantID: 101,
		StoreID:  "store-1",
		Name:     "Store",
	})
	assertOwnerlessWriteRejected(t, db, "listing_store", err)
}

func TestCreateStorePersistsTrustedInternalOwner(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingStore{})
	created, err := NewGormStoreRepository(db).CreateStore(WithOwnerUserID(context.Background(), " internal-sub "), &Store{
		TenantID: 101,
		StoreID:  "store-1",
		Name:     "Store",
	})
	if err != nil {
		t.Fatalf("CreateStore() error = %v", err)
	}
	if created.OwnerUserID != "internal-sub" {
		t.Fatalf("created owner = %q, want internal-sub", created.OwnerUserID)
	}
}

func TestCreateCategoryRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingCategory{})
	_, err := NewGormCategoryRepository(db).CreateCategory(context.Background(), &Category{
		TenantID: 101,
		Name:     "Category",
		Code:     "category-1",
	})
	assertOwnerlessWriteRejected(t, db, "listing_category", err)
}

func TestCreateFilterRuleRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingFilterRule{})
	_, err := NewGormFilterRuleRepository(db).CreateFilterRule(context.Background(), &FilterRule{
		TenantID: 101,
		Name:     "Rule",
		RuleCode: "rule-1",
	})
	assertOwnerlessWriteRejected(t, db, "listing_filter_rule", err)
}

func TestCreateGenerationTopicPolicyRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingGenerationTopicPolicy{})
	_, err := NewGormGenerationTopicPolicyRepository(db).CreateGenerationTopicPolicy(context.Background(), &GenerationTopicPolicy{
		TenantID: 101,
		Platform: "SHEIN",
		TopicKey: "title",
	})
	assertOwnerlessWriteRejected(t, db, "listing_generation_topic_policy", err)
}

func TestCreateGenerationTopicOverrideRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingGenerationTopicOverride{})
	_, err := NewGormGenerationTopicOverrideRepository(db).CreateGenerationTopicOverride(context.Background(), &GenerationTopicOverride{
		TenantID: 101,
		Platform: "SHEIN",
		TopicKey: "title",
	})
	assertOwnerlessWriteRejected(t, db, "listing_generation_topic_override", err)
}

func TestCreateOperationStrategyRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingOperationStrategy{})
	_, err := NewGormOperationStrategyRepository(db).CreateOperationStrategy(context.Background(), &OperationStrategy{
		TenantID: 101,
		StoreID:  1,
		Name:     "Strategy",
	})
	assertOwnerlessWriteRejected(t, db, "listing_operation_strategy", err)
}

func TestCreatePricingRuleRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingPricingRule{})
	_, err := NewGormPricingRuleRepository(db).CreatePricingRule(context.Background(), &PricingRule{
		TenantID: 101,
		Name:     "Rule",
		RuleCode: "rule-1",
		RuleType: "fixed",
	})
	assertOwnerlessWriteRejected(t, db, "listing_pricing_rule", err)
}

func TestCreateProfitRuleRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingProfitRule{})
	_, err := NewGormProfitRuleRepository(db).CreateProfitRule(context.Background(), &ProfitRule{
		TenantID: 101,
		Name:     "Rule",
		RuleCode: "rule-1",
	})
	assertOwnerlessWriteRejected(t, db, "listing_profit_rule", err)
}

func TestUpsertScheduledTaskConfigRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingScheduledTaskConfig{})
	_, err := NewGormScheduledTaskConfigRepository(db).UpsertScheduledTaskConfig(context.Background(), &ScheduledTaskConfig{
		TenantID:        101,
		StoreID:         1,
		Platform:        "SHEIN",
		TaskType:        "sync",
		IntervalSeconds: 3600,
	})
	assertOwnerlessWriteRejected(t, db, "listing_scheduled_task_config", err)
}

func TestCreateSensitiveWordRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingSensitiveWord{})
	_, err := NewGormSensitiveWordRepository(db).CreateSensitiveWord(context.Background(), &SensitiveWord{
		TenantID: 101,
		Word:     "blocked",
		Language: "en",
	})
	assertOwnerlessWriteRejected(t, db, "listing_sensitive_word", err)
}
