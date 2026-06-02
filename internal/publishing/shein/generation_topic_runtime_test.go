package shein

import (
	"context"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit/tenantctx"
)

type stubGenerationTopicPolicyRepository struct {
	keys map[int64][]string
	err  error
}

func (s *stubGenerationTopicPolicyRepository) ListGenerationTopicPolicies(context.Context, listingadmin.GenerationTopicPolicyQuery) (*listingadmin.GenerationTopicPolicyPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicPolicyRepository) ListEnabledTopicKeys(_ context.Context, tenantID int64, platform string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	if !strings.EqualFold(strings.TrimSpace(platform), generationTopicPolicyPlatformShein) {
		return nil, nil
	}
	return append([]string(nil), s.keys[tenantID]...), nil
}

func (s *stubGenerationTopicPolicyRepository) CreateGenerationTopicPolicy(context.Context, *listingadmin.GenerationTopicPolicy) (*listingadmin.GenerationTopicPolicy, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicPolicyRepository) GetGenerationTopicPolicy(context.Context, int64, int64) (*listingadmin.GenerationTopicPolicy, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicPolicyRepository) UpdateGenerationTopicPolicy(context.Context, *listingadmin.GenerationTopicPolicy) (*listingadmin.GenerationTopicPolicy, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicPolicyRepository) UpdateGenerationTopicPolicyStatus(context.Context, int64, int64, int16, string) (*listingadmin.GenerationTopicPolicy, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicPolicyRepository) DeleteGenerationTopicPolicy(context.Context, int64, int64) error {
	return errors.New("not implemented")
}

func TestLoadTenantGenerationTopicPolicySummaryBuildsDirectiveSummary(t *testing.T) {
	restore := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: []string{"rock", "children", "baby"},
		},
	})
	defer restore()

	ctx := tenantctx.WithTenantID(context.Background(), "101")
	summary := loadTenantGenerationTopicPolicySummary(ctx)
	if !strings.Contains(summary, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("summary = %q, want children directive", summary)
	}
	if !strings.Contains(summary, "Do not mention babies, newborns, or infant-specific usage.") {
		t.Fatalf("summary = %q, want baby directive", summary)
	}
	if strings.Contains(summary, "rock") {
		t.Fatalf("summary = %q, want unknown keys ignored", summary)
	}
}

func TestLoadTenantGenerationTopicPolicySummaryReturnsEmptyWhenTenantOrRepoMissing(t *testing.T) {
	restore := SetGenerationTopicPolicyRepository(nil)
	defer restore()

	if summary := loadTenantGenerationTopicPolicySummary(context.Background()); summary != "" {
		t.Fatalf("summary without repo/tenant = %q, want empty", summary)
	}

	restore = SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: []string{"rock"},
		},
	})
	defer restore()

	if summary := loadTenantGenerationTopicPolicySummary(context.Background()); summary != "" {
		t.Fatalf("summary without tenant = %q, want empty", summary)
	}
}

func TestTenantGenerationTopicPolicyPromptBlockFormatsSummary(t *testing.T) {
	restore := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: []string{"children"},
		},
	})
	defer restore()

	block := tenantGenerationTopicPolicyPromptBlock(tenantctx.WithTenantID(context.Background(), "101"))
	if !strings.Contains(block, "Additional tenant content restrictions:") {
		t.Fatalf("prompt block = %q, want restrictions header", block)
	}
	if !strings.Contains(block, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("prompt block = %q, want directive summary", block)
	}
}
