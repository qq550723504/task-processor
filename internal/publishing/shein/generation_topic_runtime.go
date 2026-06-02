package shein

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit/tenantctx"
	generationtopics "task-processor/internal/shein/generationtopics"
)

const generationTopicPolicyPlatformShein = "shein"

var (
	generationTopicPolicyRepoMu   sync.RWMutex
	generationTopicPolicyRepo     listingadmin.GenerationTopicPolicyRepository
	generationTopicOverrideRepoMu sync.RWMutex
	generationTopicOverrideRepo   listingadmin.GenerationTopicOverrideRepository
)

func SetGenerationTopicPolicyRepository(repo listingadmin.GenerationTopicPolicyRepository) func() {
	generationTopicPolicyRepoMu.Lock()
	previous := generationTopicPolicyRepo
	generationTopicPolicyRepo = repo
	generationTopicPolicyRepoMu.Unlock()
	return func() {
		generationTopicPolicyRepoMu.Lock()
		generationTopicPolicyRepo = previous
		generationTopicPolicyRepoMu.Unlock()
	}
}

func currentGenerationTopicPolicyRepository() listingadmin.GenerationTopicPolicyRepository {
	generationTopicPolicyRepoMu.RLock()
	repo := generationTopicPolicyRepo
	generationTopicPolicyRepoMu.RUnlock()
	return repo
}

func SetGenerationTopicOverrideRepository(repo listingadmin.GenerationTopicOverrideRepository) func() {
	generationTopicOverrideRepoMu.Lock()
	previous := generationTopicOverrideRepo
	generationTopicOverrideRepo = repo
	generationTopicOverrideRepoMu.Unlock()
	return func() {
		generationTopicOverrideRepoMu.Lock()
		generationTopicOverrideRepo = previous
		generationTopicOverrideRepoMu.Unlock()
	}
}

func currentGenerationTopicOverrideRepository() listingadmin.GenerationTopicOverrideRepository {
	generationTopicOverrideRepoMu.RLock()
	repo := generationTopicOverrideRepo
	generationTopicOverrideRepoMu.RUnlock()
	return repo
}

func loadTenantGenerationTopicPolicySummary(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	repo := currentGenerationTopicPolicyRepository()
	if repo == nil {
		return ""
	}
	tenantID, ok := tenantIDFromContext(ctx)
	if !ok {
		return ""
	}
	keys, err := repo.ListEnabledTopicKeys(ctx, tenantID, generationTopicPolicyPlatformShein)
	if err != nil || len(keys) == 0 {
		return ""
	}
	definitions := loadTenantGenerationTopicDefinitions(ctx, tenantID, keys)
	if len(definitions) == 0 {
		return ""
	}
	return generationtopics.AssembleSheinPolicySummary(
		definitions,
		generationtopics.SheinGenerationPolicyMaxDirectives,
		generationtopics.SheinGenerationPolicyMaxChars,
	)
}

func loadTenantGenerationTopicDefinitions(ctx context.Context, tenantID int64, topicKeys []string) []generationtopics.Definition {
	overrideRepo := currentGenerationTopicOverrideRepository()
	if overrideRepo == nil {
		definitions, _ := generationtopics.ResolveSheinTopicKeys(topicKeys)
		return definitions
	}
	return generationtopics.ResolveSheinTopicDefinitionsWithOverlay(topicKeys, func(topicKey string) (generationtopics.DefinitionOverlay, error) {
		override, err := overrideRepo.GetGenerationTopicOverrideByTopicKey(ctx, tenantID, generationTopicPolicyPlatformShein, topicKey)
		if err != nil || override == nil || override.Status != 1 {
			return generationtopics.DefinitionOverlay{}, err
		}
		return generationtopics.DefinitionOverlay{
			Enabled:           true,
			PromptDirectives:  override.AdditionalPromptDirectives,
			LexiconByLanguage: override.AdditionalLexiconByLanguage,
		}, nil
	})
}

func tenantIDFromContext(ctx context.Context) (int64, bool) {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if !ok {
		return 0, false
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" || tenantID == tenantctx.DefaultTenantID {
		return 0, false
	}
	parsed, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func tenantGenerationTopicPolicyPromptBlock(ctx context.Context) string {
	summary := loadTenantGenerationTopicPolicySummary(ctx)
	if summary == "" {
		return ""
	}
	return fmt.Sprintf("\n\nAdditional tenant content restrictions:\n%s", summary)
}
