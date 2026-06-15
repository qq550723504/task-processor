package submitprep

import (
	"context"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/listingadmin"
	sharedtenantctx "task-processor/internal/shared/tenantctx"
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
	if !strings.EqualFold(strings.TrimSpace(platform), "shein") {
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

func TestNewSensitiveWordServiceForContext_OverlaysTenantGenerationTopicLexicon(t *testing.T) {
	restoreRepo := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: {"children", "unknown"},
		},
	})
	defer restoreRepo()

	service := NewSensitiveWordServiceForContext(sharedtenantctx.WithTenantID(context.Background(), "101"))
	got := service.SanitizeDisplayTextWithContext(nil, "Kids room curtain for home decor")
	if strings.Contains(strings.ToLower(got), "kids") {
		t.Fatalf("sanitized text = %q, want children topic lexicon removed", got)
	}
}
