package management

import (
	"errors"
	"strings"
)

// LocalListingRuntimeReport summarizes whether listing runtime can run without
// falling back to Java management HTTP endpoints.
type LocalListingRuntimeReport struct {
	Ready                bool
	DB                   bool
	Redis                bool
	ImportTask           bool
	Store                bool
	ProductImportMapping bool
	ProductData          bool
	FilterRule           bool
	ProfitRule           bool
	PricingRule          bool
	DailyQuota           bool
}

func (r LocalListingRuntimeReport) Fields() map[string]bool {
	return map[string]bool{
		"ready":                  r.Ready,
		"db":                     r.DB,
		"redis":                  r.Redis,
		"import_task":            r.ImportTask,
		"store":                  r.Store,
		"product_import_mapping": r.ProductImportMapping,
		"product_data":           r.ProductData,
		"filter_rule":            r.FilterRule,
		"profit_rule":            r.ProfitRule,
		"pricing_rule":           r.PricingRule,
		"daily_quota":            r.DailyQuota,
	}
}

func (cm *ClientManager) ValidateLocalListingRuntimeFields() (map[string]bool, error) {
	report, err := cm.ValidateLocalListingRuntime()
	return report.Fields(), err
}

func (cm *ClientManager) ValidateLocalListingRuntime() (LocalListingRuntimeReport, error) {
	var report LocalListingRuntimeReport
	if cm == nil {
		return report, errors.New("management client is not initialized")
	}

	cm.mutex.RLock()
	provider := cm.localDataProvider
	cm.mutex.RUnlock()
	if provider == nil {
		return report, errors.New("local management data provider is not configured")
	}

	report.DB = provider.HasDB()
	report.Redis = provider.HasRedis()
	report.ImportTask = provider.importTaskRepository() != nil
	report.Store = provider.storeRepository() != nil
	report.ProductImportMapping = provider.productImportMappingRepository() != nil
	report.ProductData = provider.ProductDataRepository() != nil
	report.FilterRule = provider.filterRuleRepository() != nil
	report.ProfitRule = provider.profitRuleRepository() != nil
	report.PricingRule = provider.pricingRuleRepository() != nil
	report.DailyQuota = report.Redis
	report.Ready = report.DB &&
		report.Redis &&
		report.ImportTask &&
		report.Store &&
		report.ProductImportMapping &&
		report.ProductData &&
		report.FilterRule &&
		report.ProfitRule &&
		report.PricingRule &&
		report.DailyQuota

	if report.Ready {
		return report, nil
	}

	missing := missingLocalListingRuntimeCapabilities(report)
	return report, errors.New("local listing runtime is not ready: " + strings.Join(missing, ", "))
}

func missingLocalListingRuntimeCapabilities(report LocalListingRuntimeReport) []string {
	missing := make([]string, 0)
	if !report.DB {
		missing = append(missing, "local database is not configured")
	}
	if !report.Redis {
		missing = append(missing, "local redis is not configured")
	}
	if !report.ImportTask {
		missing = append(missing, "import task repository is not configured")
	}
	if !report.Store {
		missing = append(missing, "store repository is not configured")
	}
	if !report.ProductImportMapping {
		missing = append(missing, "product import mapping repository is not configured")
	}
	if !report.ProductData {
		missing = append(missing, "product data repository is not configured")
	}
	if !report.FilterRule {
		missing = append(missing, "filter rule repository is not configured")
	}
	if !report.ProfitRule {
		missing = append(missing, "profit rule repository is not configured")
	}
	if !report.PricingRule {
		missing = append(missing, "pricing rule repository is not configured")
	}
	if !report.DailyQuota {
		missing = append(missing, "daily quota runtime is not configured")
	}
	return missing
}
