package listingadmin

import (
	"context"
	"testing"
)

func TestGormGenerationTopicPolicyRepository_ListEnabledTopicKeysByTenantAndPlatform(t *testing.T) {
	db := openGenerationTopicPolicyTestDB(t)
	if err := AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		t.Fatalf("AutoMigrateGenerationTopicPolicyRepository: %v", err)
	}

	repo := NewGormGenerationTopicPolicyRepository(db)
	createCtx := withRequestIdentity(context.TODO(), "planner", nil)
	readCtx := withRequestIdentity(context.TODO(), "reviewer", nil)

	for _, policy := range []GenerationTopicPolicy{
		{TenantID: 101, Platform: "shein", TopicKey: "children", Status: 1},
		{TenantID: 101, Platform: "shein", TopicKey: "food", Status: 0},
		{TenantID: 101, Platform: "amazon", TopicKey: "children", Status: 1},
		{TenantID: 202, Platform: "shein", TopicKey: "children", Status: 1},
	} {
		if _, err := repo.CreateGenerationTopicPolicy(createCtx, &policy); err != nil {
			t.Fatalf("CreateGenerationTopicPolicy(%+v): %v", policy, err)
		}
	}

	keys, err := repo.ListEnabledTopicKeys(readCtx, 101, "shein")
	if err != nil {
		t.Fatalf("ListEnabledTopicKeys: %v", err)
	}
	if len(keys) != 1 || keys[0] != "children" {
		t.Fatalf("keys = %#v, want []string{\"children\"}", keys)
	}
}

func TestGormGenerationTopicPolicyRepository_ListEnabledTopicKeysIgnoresOwnerScope(t *testing.T) {
	db := openGenerationTopicPolicyTestDB(t)
	if err := AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		t.Fatalf("AutoMigrateGenerationTopicPolicyRepository: %v", err)
	}

	repo := NewGormGenerationTopicPolicyRepository(db)
	createCtx := withRequestIdentity(context.TODO(), "planner", nil)
	readCtx := withRequestIdentity(context.TODO(), "other-user", nil)

	if _, err := repo.CreateGenerationTopicPolicy(createCtx, &GenerationTopicPolicy{
		TenantID: 101,
		Platform: "shein",
		TopicKey: "children",
		Status:   1,
	}); err != nil {
		t.Fatalf("CreateGenerationTopicPolicy: %v", err)
	}

	keys, err := repo.ListEnabledTopicKeys(readCtx, 101, "shein")
	if err != nil {
		t.Fatalf("ListEnabledTopicKeys: %v", err)
	}
	if len(keys) != 1 || keys[0] != "children" {
		t.Fatalf("keys = %#v, want tenant-global access across users", keys)
	}
}
