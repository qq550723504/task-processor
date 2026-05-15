package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingsubscription"
)

func main() {
	var (
		configPath  = flag.String("config", "config/config-dev.yaml", "config file path")
		tenantID    = flag.String("tenant-id", "", "ZITADEL resource owner id")
		modulesCSV  = flag.String("modules", "", "comma-separated module codes")
		status      = flag.String("status", listingsubscription.StatusActive, "entitlement status")
		limitsJSON  = flag.String("limits-json", "{}", "limits JSON object")
		startsAtRaw = flag.String("starts-at", "", "RFC3339 start time")
		expiresRaw  = flag.String("expires-at", "", "RFC3339 expiry time")
		actorID     = flag.String("actor-id", "cli", "audit actor id")
		reason      = flag.String("reason", "cli subscription update", "audit reason")
		usageModule = flag.String("usage-module", "", "module code for usage adjustment")
		usagePeriod = flag.String("usage-period", "", "usage period key, defaults to current YYYY-MM")
		usageMetric = flag.String("usage-metric", "", "usage metric to set")
		usageUsed   = flag.Int("usage-used", -1, "usage value to set; set >=0 to apply")
	)
	flag.Parse()

	if strings.TrimSpace(*tenantID) == "" {
		exitf("tenant-id is required")
	}

	cfg, err := config.LoadConfigFromFile(*configPath)
	if err != nil {
		exitf("load config: %v", err)
	}
	db, err := database.NewDatabaseFromConfig(cfg.Database)
	if err != nil {
		exitf("connect database: %v", err)
	}
	if db == nil {
		exitf("database is not configured")
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}
	if err := listingsubscription.AutoMigrateRepository(db); err != nil {
		exitf("migrate subscription tables: %v", err)
	}
	service, err := listingsubscription.NewService(listingsubscription.NewGormRepository(db))
	if err != nil {
		exitf("create subscription service: %v", err)
	}

	ctx := context.Background()
	limits, err := parseLimits(*limitsJSON)
	if err != nil {
		exitf("parse limits-json: %v", err)
	}
	startsAt, err := parseOptionalTime(*startsAtRaw)
	if err != nil {
		exitf("parse starts-at: %v", err)
	}
	expiresAt, err := parseOptionalTime(*expiresRaw)
	if err != nil {
		exitf("parse expires-at: %v", err)
	}
	for _, moduleCode := range splitCSV(*modulesCSV) {
		entitlement, err := service.UpsertEntitlementWithAudit(ctx, *tenantID, moduleCode, listingsubscription.EntitlementInput{
			Status:    *status,
			StartsAt:  startsAt,
			ExpiresAt: expiresAt,
			Limits:    limits,
		}, *actorID, *reason)
		if err != nil {
			exitf("upsert entitlement %s: %v", moduleCode, err)
		}
		fmt.Printf("entitlement updated tenant=%s module=%s status=%s\n", entitlement.TenantID, entitlement.ModuleCode, entitlement.Status)
	}
	if *usageUsed >= 0 {
		moduleCode := strings.TrimSpace(*usageModule)
		if moduleCode == "" {
			exitf("usage-module is required when usage-used is set")
		}
		metric := strings.TrimSpace(*usageMetric)
		if metric == "" {
			exitf("usage-metric is required when usage-used is set")
		}
		period := strings.TrimSpace(*usagePeriod)
		if period == "" {
			period = time.Now().UTC().Format("2006-01")
		}
		counter, err := service.SetUsage(ctx, *tenantID, moduleCode, listingsubscription.UsageAdjustmentInput{
			PeriodKey: period,
			Metric:    metric,
			Used:      *usageUsed,
			Reason:    *reason,
		}, *actorID)
		if err != nil {
			exitf("set usage: %v", err)
		}
		fmt.Printf("usage updated tenant=%s module=%s period=%s metric=%s used=%d\n", counter.TenantID, counter.ModuleCode, counter.PeriodKey, counter.Metric, counter.Used)
	}
	if len(splitCSV(*modulesCSV)) == 0 && *usageUsed < 0 {
		exitf("nothing to do: set modules or usage-used")
	}
}

func parseLimits(value string) (map[string]int, error) {
	var limits map[string]int
	if err := json.Unmarshal([]byte(value), &limits); err != nil {
		return nil, err
	}
	if limits == nil {
		return map[string]int{}, nil
	}
	for key, count := range limits {
		if strings.TrimSpace(key) == "" || count < 0 {
			return nil, fmt.Errorf("invalid limit %q=%d", key, count)
		}
	}
	return limits, nil
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func splitCSV(value string) []string {
	out := []string{}
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
