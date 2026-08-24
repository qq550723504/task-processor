package httpapi

import (
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
)

type generationUsageTenantCohort map[string]struct{}

func generationUsageAdmissionForConfig(cfg *config.Config) listingkit.GenerationUsageAdmission {
	if cfg == nil || !cfg.ListingKit.GenerationUsageLedgerEnabled {
		return generationUsageTenantCohort{}
	}
	cohort := make(generationUsageTenantCohort, len(cfg.ListingKit.GenerationUsageLedgerTenantIDs))
	for _, tenantID := range cfg.ListingKit.GenerationUsageLedgerTenantIDs {
		if normalized := strings.TrimSpace(tenantID); normalized != "" {
			cohort[normalized] = struct{}{}
		}
	}
	return cohort
}

func (c generationUsageTenantCohort) AllowsGenerationUsage(tenantID string) bool {
	_, allowed := c[strings.TrimSpace(tenantID)]
	return allowed
}
