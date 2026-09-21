package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/aicapability"
	aicapabilitystore "task-processor/internal/aicapability/store"
	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	"task-processor/internal/listingsubscription"
	platformdatabase "task-processor/internal/platform/database"
)

// This is an owner-triggered recovery command, not a retry runner. It closes
// one durable dispatch after an operator has independently confirmed the
// provider outcome. It never calls the provider.
func main() {
	configPath := flag.String("config", "config/config-prod.yaml", "config file path")
	tenantID := flag.String("tenant-id", "", "organization/tenant ID")
	memberID := flag.String("member-id", "", "organization member ID")
	invocationID := flag.String("invocation-id", "", "durable AI invocation ID")
	outcome := flag.String("outcome", "", "confirmed provider outcome: succeeded or failed")
	providerRequestID := flag.String("provider-request-id", "", "provider request reference required for a confirmed success")
	promptTokens := flag.Int("prompt-tokens", 0, "provider-observed prompt tokens for a confirmed success")
	completionTokens := flag.Int("completion-tokens", 0, "provider-observed completion tokens for a confirmed success")
	finishedAt := flag.String("finished-at", "", "provider completion time in RFC3339; defaults to now")
	flag.Parse()

	if err := run(context.Background(), runOptions{ConfigPath: *configPath, TenantID: *tenantID, MemberID: *memberID, InvocationID: *invocationID, Outcome: *outcome, ProviderRequestID: *providerRequestID, PromptTokens: *promptTokens, CompletionTokens: *completionTokens, FinishedAt: *finishedAt}); err != nil {
		log.Fatal(err)
	}
}

type runOptions struct {
	ConfigPath, TenantID, MemberID, InvocationID, Outcome, ProviderRequestID, FinishedAt string
	PromptTokens, CompletionTokens                                                       int
}

func run(ctx context.Context, options runOptions) error {
	if strings.TrimSpace(options.TenantID) == "" || strings.TrimSpace(options.MemberID) == "" || strings.TrimSpace(options.InvocationID) == "" {
		return fmt.Errorf("tenant-id, member-id, and invocation-id are required")
	}
	if options.Outcome != string(aicapability.InvocationSucceeded) && options.Outcome != string(aicapability.InvocationFailed) {
		return fmt.Errorf("outcome must be succeeded or failed")
	}
	if options.PromptTokens < 0 || options.CompletionTokens < 0 {
		return fmt.Errorf("observed token counts must not be negative")
	}
	finished := time.Now().UTC()
	if strings.TrimSpace(options.FinishedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, options.FinishedAt)
		if err != nil {
			return fmt.Errorf("parse finished-at: %w", err)
		}
		finished = parsed.UTC()
	}
	cfg, err := config.LoadConfigFromFileWithoutValidation(options.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil || cfg.Database == nil || cfg.CommercialDatabase == nil {
		return fmt.Errorf("worker and commercial database configurations are required")
	}
	invocationDB, err := platformdatabase.Open(configadapter.Database(cfg.Database))
	if err != nil {
		return fmt.Errorf("open invocation database: %w", err)
	}
	defer closeDatabase(invocationDB)
	commercialDB, err := platformdatabase.Open(configadapter.Database(cfg.CommercialDatabase))
	if err != nil {
		return fmt.Errorf("open commercial database: %w", err)
	}
	defer closeDatabase(commercialDB)
	recorder := aicapabilitystore.NewGormInvocationRecorder(invocationDB)
	recorder.SetUsageSettler(listingsubscription.AIInvocationUsageAdapter{Repository: listingsubscription.NewGormRepository(commercialDB)})
	record := aicapability.InvocationRecord{InvocationID: options.InvocationID, TenantID: options.TenantID, MemberID: options.MemberID, UserID: options.MemberID, FinishedAt: finished, Outcome: aicapability.InvocationOutcome(options.Outcome), ProviderRequestID: options.ProviderRequestID}
	if record.Outcome == aicapability.InvocationSucceeded {
		if strings.TrimSpace(options.ProviderRequestID) == "" {
			return fmt.Errorf("provider-request-id is required for a confirmed success")
		}
		total := options.PromptTokens + options.CompletionTokens
		if total <= 0 {
			return fmt.Errorf("successful recovery requires positive observed token counts")
		}
		record.PromptTokens, record.CompletionTokens, record.TotalTokens, record.UsageKnown = options.PromptTokens, options.CompletionTokens, total, true
	} else {
		record.ErrorCode = "provider_outcome_confirmed_failed"
	}
	return recorder.ResolveDispatchedInvocation(ctx, record)
}

func closeDatabase(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
