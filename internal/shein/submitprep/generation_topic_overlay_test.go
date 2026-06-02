package submitprep

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	if !strings.EqualFold(strings.TrimSpace(platform), "shein") {
		return nil, nil
	}
	return append([]string(nil), s.keys[tenantID]...), nil
}

func (s *stubGenerationTopicPolicyRepository) CreateGenerationTopicPolicy(context.Context, *listingadmin.GenerationTopicPolicy) (*listingadmin.GenerationTopicPolicy, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTopicPolicyRepository) UpdateGenerationTopicPolicy(context.Context, *listingadmin.GenerationTopicPolicy) (*listingadmin.GenerationTopicPolicy, error) {
	return nil, errors.New("not implemented")
}

func TestNewSensitiveWordServiceForContext_OverlaysTenantGenerationTopicLexicon(t *testing.T) {
	restoreConfig := writeSensitiveWordsConfigForOverlayTest(t, `{
  "static_words": {},
  "dynamic_words": {},
  "last_updated": "2026-06-02T00:00:00Z",
  "version": "1.0.0",
  "platform": "shein"
}`)
	defer restoreConfig()

	restoreRepo := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: {"children", "unknown"},
		},
	})
	defer restoreRepo()

	service := NewSensitiveWordServiceForContext(tenantctx.WithTenantID(context.Background(), "101"))
	got := service.SanitizeDisplayTextWithContext(nil, "Kids room curtain for home decor")
	if strings.Contains(strings.ToLower(got), "kids") {
		t.Fatalf("sanitized text = %q, want children topic lexicon removed", got)
	}
}

func writeSensitiveWordsConfigForOverlayTest(t *testing.T, contents string) func() {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sensitive_words_shein.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write sensitive words config: %v", err)
	}
	return SetSensitiveWordsConfigPathForTesting(path)
}
