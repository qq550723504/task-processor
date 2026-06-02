package shein

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit/tenantctx"
)

type stubGenerationTopicPolicyRepository struct {
	keys map[int64][]string
	err  error
}

type stubGenerationTopicOverrideRepository struct {
	items map[string]listingadmin.GenerationTopicOverride
	err   error
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

func (s *stubGenerationTopicOverrideRepository) ListGenerationTopicOverrides(context.Context, listingadmin.GenerationTopicOverrideQuery) (*listingadmin.GenerationTopicOverridePage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicOverrideRepository) GetGenerationTopicOverride(context.Context, int64, int64) (*listingadmin.GenerationTopicOverride, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicOverrideRepository) GetGenerationTopicOverrideByTopicKey(_ context.Context, tenantID int64, platform string, topicKey string) (*listingadmin.GenerationTopicOverride, error) {
	if s.err != nil {
		return nil, s.err
	}
	item, ok := s.items[overrideRepoKey(tenantID, platform, topicKey)]
	if !ok {
		return nil, listingadmin.ErrGenerationTopicOverrideNotFound
	}
	cloned := item
	return &cloned, nil
}

func (s *stubGenerationTopicOverrideRepository) CreateGenerationTopicOverride(context.Context, *listingadmin.GenerationTopicOverride) (*listingadmin.GenerationTopicOverride, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicOverrideRepository) UpdateGenerationTopicOverride(context.Context, *listingadmin.GenerationTopicOverride) (*listingadmin.GenerationTopicOverride, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicOverrideRepository) UpdateGenerationTopicOverrideStatus(context.Context, int64, int64, int16, string) (*listingadmin.GenerationTopicOverride, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicOverrideRepository) DeleteGenerationTopicOverride(context.Context, int64, int64) error {
	return errors.New("not implemented")
}

func overrideRepoKey(tenantID int64, platform string, topicKey string) string {
	return strings.Join([]string{
		strings.TrimSpace(strconv.FormatInt(tenantID, 10)),
		strings.TrimSpace(strings.ToLower(platform)),
		strings.TrimSpace(strings.ToLower(topicKey)),
	}, ":")
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

func TestLoadTenantGenerationTopicPolicySummaryIncludesOverrideDirectives(t *testing.T) {
	restorePolicyRepo := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: {"children"},
		},
	})
	defer restorePolicyRepo()
	restoreOverrideRepo := SetGenerationTopicOverrideRepository(&stubGenerationTopicOverrideRepository{
		items: map[string]listingadmin.GenerationTopicOverride{
			overrideRepoKey(101, "shein", "children"): {
				TenantID:                   101,
				Platform:                   "shein",
				TopicKey:                   "children",
				AdditionalPromptDirectives: []string{"Avoid toddler-focused positioning."},
				Status:                     1,
			},
		},
	})
	defer restoreOverrideRepo()

	summary := loadTenantGenerationTopicPolicySummary(tenantctx.WithTenantID(context.Background(), "101"))
	if !strings.Contains(summary, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("summary = %q, want default directive retained", summary)
	}
	if !strings.Contains(summary, "Avoid toddler-focused positioning.") {
		t.Fatalf("summary = %q, want override directive included", summary)
	}
}
