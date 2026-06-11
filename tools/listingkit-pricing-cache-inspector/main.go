package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type taskRow struct {
	ID        string `gorm:"column:id"`
	Status    string `gorm:"column:status"`
	Request   string `gorm:"column:request"`
	Result    string `gorm:"column:result"`
	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`
}

type generateRequest struct {
	SheinStoreID int64 `json:"shein_store_id,omitempty"`
}

type listingKitResult struct {
	Shein *sheinPackage `json:"shein,omitempty"`
}

type sheinPackage struct {
	SpuName           string            `json:"spu_name,omitempty"`
	ProductNameEn     string            `json:"product_name_en,omitempty"`
	BrandName         string            `json:"brand_name,omitempty"`
	CategoryID        int               `json:"category_id,omitempty"`
	CategoryIDList    []int             `json:"category_id_list,omitempty"`
	CategoryPath      []string          `json:"category_path,omitempty"`
	ProductAttributes []namedAttribute  `json:"product_attributes,omitempty"`
	DraftPayload      *requestDraft     `json:"draft_payload,omitempty"`
	Pricing           *pricingReview    `json:"pricing,omitempty"`
	SubmissionEvents  []submissionEvent `json:"submission_events,omitempty"`
}

type namedAttribute struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type requestDraft struct {
	SupplierCode string            `json:"supplier_code,omitempty"`
	SKCList      []skcRequestDraft `json:"skc_list,omitempty"`
}

type skcRequestDraft struct {
	SupplierCode string     `json:"supplier_code,omitempty"`
	SKUList      []skuDraft `json:"sku_list,omitempty"`
}

type skuDraft struct {
	SupplierSKU   string            `json:"supplier_sku,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	Currency      string            `json:"currency,omitempty"`
	CostPrice     string            `json:"cost_price,omitempty"`
	SitePriceList []sitePrice       `json:"site_price_list,omitempty"`
}

type sitePrice struct {
	Currency string `json:"currency,omitempty"`
}

type pricingReview struct {
	SKUPrices        []skuPriceReview    `json:"sku_prices,omitempty"`
	ManualOverrides  map[string]float64  `json:"manual_overrides,omitempty"`
	Cache            *resolutionCacheRef `json:"cache,omitempty"`
	Ready            bool                `json:"ready"`
	UpdatedAt        string              `json:"updated_at,omitempty"`
	MissingPriceSKUs []string            `json:"missing_price_skus,omitempty"`
}

type skuPriceReview struct {
	SupplierSKU  string  `json:"supplier_sku,omitempty"`
	SupplierCode string  `json:"supplier_code,omitempty"`
	CostCNY      float64 `json:"cost_cny,omitempty"`
	FinalPrice   float64 `json:"final_price,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	Manual       bool    `json:"manual,omitempty"`
}

type resolutionCacheRef struct {
	Status    string `json:"status,omitempty"`
	Source    string `json:"source,omitempty"`
	HitSource string `json:"hit_source,omitempty"`
	CacheKey  string `json:"cache_key,omitempty"`
	ShortKey  string `json:"short_key,omitempty"`
}

type submissionEvent struct {
	Action    string `json:"action,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type cacheRow struct {
	CacheKind      string `gorm:"column:cache_kind"`
	CacheKey       string `gorm:"column:cache_key"`
	ShortKey       string `gorm:"column:short_key"`
	Source         string `gorm:"column:source"`
	Manual         bool   `gorm:"column:manual"`
	HitCount       int    `gorm:"column:hit_count"`
	UpdatedAt      string `gorm:"column:updated_at"`
	SourceIdentity string `gorm:"column:source_identity"`
}

func main() {
	var dsn string
	var latestLimit int
	flag.StringVar(&dsn, "dsn", "host=127.0.0.1 port=15432 user=postgres password=123456 dbname=ruoyi-vue-pro sslmode=disable", "Postgres DSN")
	flag.IntVar(&latestLimit, "latest-cache-limit", 5, "How many latest pricing cache rows to print")
	flag.Parse()

	taskIDs := flag.Args()
	if len(taskIDs) == 0 {
		printUsage()
		os.Exit(1)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接数据库失败: %v\n", err)
		os.Exit(1)
	}

	for _, taskID := range taskIDs {
		if err := inspectTask(db, taskID); err != nil {
			fmt.Fprintf(os.Stderr, "检查任务 %s 失败: %v\n", taskID, err)
			os.Exit(1)
		}
	}

	if latestLimit > 0 {
		if err := printLatestPricingCacheRows(db, latestLimit); err != nil {
			fmt.Fprintf(os.Stderr, "读取价格缓存失败: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Print(`ListingKit SHEIN 价格缓存排查工具

用法:
  go run ./listingkit-pricing-cache-inspector -- <task-id> [task-id...]

选项:
  --dsn <dsn>                  Postgres 连接串
  --latest-cache-limit <n>     额外打印最近 n 条 pricing cache 记录，默认 5

示例:
  go run ./listingkit-pricing-cache-inspector -- e3d18115-2fe6-4322-aecb-51fc3492842e e5e8e7c6-668c-442a-b343-370b7699944e
`)
}

func inspectTask(db *gorm.DB, taskID string) error {
	var row taskRow
	if err := db.Table("listing_kit_tasks").Where("id = ?", taskID).First(&row).Error; err != nil {
		return err
	}

	var req generateRequest
	if strings.TrimSpace(row.Request) != "" {
		if err := json.Unmarshal([]byte(row.Request), &req); err != nil {
			return fmt.Errorf("解析 request 失败: %w", err)
		}
	}

	var result listingKitResult
	if strings.TrimSpace(row.Result) != "" {
		if err := json.Unmarshal([]byte(row.Result), &result); err != nil {
			return fmt.Errorf("解析 result 失败: %w", err)
		}
	}

	fmt.Printf("=== %s ===\n", taskID)
	fmt.Printf("status=%s created_at=%s updated_at=%s\n", row.Status, row.CreatedAt, row.UpdatedAt)
	if result.Shein == nil {
		fmt.Print("missing shein result\n\n")
		return nil
	}

	pkg := result.Shein
	fmt.Printf("store_id=%d category_id=%d category_path=%v\n", req.SheinStoreID, pkg.CategoryID, pkg.CategoryPath)
	fmt.Printf("pricing_cache=%s\n", mustJSON(pkg.Pricing))
	fmt.Printf("stable_product_identity=%v\n", stablePricingProductIdentity(pkg))
	fmt.Printf("sku_facts=%v\n", sortedPricingSKUFacts(pkg, pricingRule{}))
	printDraftSKUs(pkg)
	fmt.Printf("computed_pricing_key=%s\n", pricingCacheKey(req.SheinStoreID, pkg, pricingRule{}))
	printSubmissionTimeline(pkg)

	cacheKey := ""
	if pkg.Pricing != nil && pkg.Pricing.Cache != nil {
		cacheKey = pkg.Pricing.Cache.CacheKey
	}
	if strings.TrimSpace(cacheKey) != "" {
		if err := printMatchingPricingCacheRow(db, cacheKey); err != nil {
			return err
		}
	}
	fmt.Println()
	return nil
}

func printSubmissionTimeline(pkg *sheinPackage) {
	if pkg == nil || len(pkg.SubmissionEvents) == 0 {
		return
	}
	fmt.Println("submission_events:")
	for _, item := range pkg.SubmissionEvents {
		fmt.Printf("- %s | action=%s status=%s message=%s\n", item.CreatedAt, item.Action, item.Status, item.Message)
	}
}

func printMatchingPricingCacheRow(db *gorm.DB, cacheKey string) error {
	var row cacheRow
	if err := db.Raw(`
		select cache_kind, cache_key, short_key, source, manual, hit_count,
		       to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS') as updated_at,
		       source_identity
		from shein_resolution_cache_entries
		where cache_kind = 'pricing' and cache_key = ?
		order by updated_at desc
		limit 1
	`, cacheKey).Scan(&row).Error; err != nil {
		return err
	}
	if strings.TrimSpace(row.CacheKey) == "" {
		fmt.Println("matching_cache_row=none")
		return nil
	}
	fmt.Printf("matching_cache_row short_key=%s source=%s manual=%v hit_count=%d updated_at=%s\n", row.ShortKey, row.Source, row.Manual, row.HitCount, row.UpdatedAt)
	fmt.Printf("matching_cache_source_identity=%s\n", row.SourceIdentity)
	return nil
}

func printLatestPricingCacheRows(db *gorm.DB, limit int) error {
	var rows []cacheRow
	if err := db.Raw(`
		select cache_kind, cache_key, short_key, source, manual, hit_count,
		       to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS') as updated_at,
		       source_identity
		from shein_resolution_cache_entries
		where cache_kind = 'pricing'
		order by updated_at desc
		limit ?
	`, limit).Scan(&rows).Error; err != nil {
		return err
	}
	fmt.Println("=== latest pricing cache rows ===")
	for _, row := range rows {
		fmt.Printf("short_key=%s source=%s manual=%v hit_count=%d updated_at=%s cache_key=%s\n", row.ShortKey, row.Source, row.Manual, row.HitCount, row.UpdatedAt, row.CacheKey)
		fmt.Printf("source_identity=%s\n", row.SourceIdentity)
	}
	fmt.Println()
	return nil
}

func printDraftSKUs(pkg *sheinPackage) {
	if pkg == nil || pkg.DraftPayload == nil {
		return
	}
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			fmt.Printf("draft_sku supplier=%s alias=%s source_sds_sku=%s attrs=%s\n",
				sku.SupplierSKU,
				pricingDraftSKUKey(&sku),
				strings.TrimSpace(sku.Attributes["source_sds_sku"]),
				mustJSON(sku.Attributes),
			)
		}
	}
}

type pricingRule struct {
	SourceCurrency string
	TargetCurrency string
}

func pricingCacheKey(storeID int64, pkg *sheinPackage, rule pricingRule) string {
	if pkg == nil || pkg.DraftPayload == nil {
		return ""
	}
	payload := map[string]any{
		"version":          1,
		"store_id":         fmt.Sprintf("%d", storeID),
		"category_id":      pkg.CategoryID,
		"category_id_list": append([]int(nil), pkg.CategoryIDList...),
		"category_path":    normalizedTextList(pkg.CategoryPath),
		"product_identity": stablePricingProductIdentity(pkg),
		"sku_facts":        sortedPricingSKUFacts(pkg, rule),
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func stablePricingProductIdentity(pkg *sheinPackage) []string {
	if pkg == nil {
		return nil
	}
	values := stablePackageIdentifiers(pkg)
	if len(values) > 0 {
		return values
	}
	return normalizeStableIdentity([]string{
		pkg.SpuName,
		pkg.ProductNameEn,
		lookupAttributeValue(pkg.ProductAttributes, "sku"),
		lookupAttributeValue(pkg.ProductAttributes, "parent sku"),
	})
}

func stablePackageIdentifiers(pkg *sheinPackage) []string {
	if pkg == nil {
		return nil
	}
	primary := normalizeStableIdentity([]string{
		lookupAttributeValue(pkg.ProductAttributes, "product_sku"),
		lookupAttributeValue(pkg.ProductAttributes, "variant_sku"),
		lookupAttributeValue(pkg.ProductAttributes, "source_sds_sku"),
		lookupAttributeValue(pkg.ProductAttributes, "sku"),
	})
	if len(primary) > 0 {
		return primary
	}

	secondary := make([]string, 0, 8)
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			for _, sku := range skc.SKUList {
				secondary = append(secondary, strings.TrimSpace(sku.Attributes["source_sds_sku"]))
			}
		}
	}
	secondary = normalizeStableIdentity(secondary)
	if len(secondary) > 0 {
		return secondary
	}

	fallback := []string{
		lookupAttributeValue(pkg.ProductAttributes, "supplier_sku"),
		lookupAttributeValue(pkg.ProductAttributes, "parent sku"),
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			fallback = append(fallback, normalizedSourceLikeSKU(skc.SupplierCode))
			for _, sku := range skc.SKUList {
				fallback = append(fallback, normalizedSourceLikeSKU(sku.SupplierSKU))
			}
		}
	}
	return normalizeStableIdentity(fallback)
}

func lookupAttributeValue(items []namedAttribute, target string) string {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.Name)) == target {
			return strings.TrimSpace(item.Value)
		}
	}
	return ""
}

func normalizeStableIdentity(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedPricingSKUFacts(pkg *sheinPackage, rule pricingRule) []string {
	facts := pricingSKUFacts(pkg, rule)
	if len(facts) == 0 {
		return nil
	}
	result := make([]string, 0, len(facts))
	for alias, fact := range facts {
		result = append(result, alias+"|cost="+fact.CostPrice+"|currency="+fact.Currency)
	}
	sort.Strings(result)
	return result
}

type pricingSKUFact struct {
	CostPrice string
	Currency  string
}

func pricingSKUFacts(pkg *sheinPackage, rule pricingRule) map[string]pricingSKUFact {
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	result := map[string]pricingSKUFact{}
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			alias := pricingDraftSKUKey(&sku)
			if alias == "" {
				continue
			}
			result[alias] = pricingSKUFact{
				CostPrice: formatMoney(parseMoney(sku.CostPrice)),
				Currency:  normalizeReviewCurrency(existingDraftCurrency(sku), rule),
			}
		}
	}
	return result
}

func pricingDraftSKUKey(sku *skuDraft) string {
	if sku == nil {
		return ""
	}
	if source := strings.TrimSpace(sku.Attributes["source_sds_sku"]); source != "" {
		return pricingSKUAlias(source)
	}
	return pricingSKUAlias(sku.SupplierSKU)
}

func pricingSKUAlias(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	if index := strings.Index(value, "-V"); index > 0 {
		return strings.TrimSpace(value[:index])
	}
	parts := strings.Split(value, "-")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if looksLikePricingRequestToken(part) {
			continue
		}
		filtered = append(filtered, part)
	}
	alias := strings.Join(filtered, "-")
	if prefix, ok := trimStyleSuffix(alias); ok {
		return prefix
	}
	return alias
}

func looksLikePricingRequestToken(token string) bool {
	token = strings.TrimSpace(strings.ToUpper(token))
	if len(token) < 6 || len(token) > 9 || !strings.HasPrefix(token, "R") {
		return false
	}
	for _, r := range token[1:] {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func trimStyleSuffix(value string) (string, bool) {
	index := strings.LastIndex(value, "-")
	if index <= 0 {
		return "", false
	}
	suffix := strings.TrimSpace(value[index+1:])
	if len(suffix) != 8 {
		return "", false
	}
	for _, r := range suffix {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return "", false
		}
	}
	prefix := strings.TrimSpace(value[:index])
	if prefix == "" || !strings.ContainsAny(prefix, "0123456789") {
		return "", false
	}
	return prefix, true
}

func normalizedSourceLikeSKU(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if index := strings.Index(lower, "-v"); index > 0 {
		value = value[:index]
	}
	if index := strings.LastIndex(value, "-"); index > 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func existingDraftCurrency(sku skuDraft) string {
	if strings.TrimSpace(sku.Currency) != "" {
		return sku.Currency
	}
	for _, site := range sku.SitePriceList {
		if strings.TrimSpace(site.Currency) != "" {
			return site.Currency
		}
	}
	return ""
}

func normalizeReviewCurrency(currency string, rule pricingRule) string {
	sourceCurrency := strings.ToUpper(strings.TrimSpace(rule.SourceCurrency))
	if sourceCurrency == "" {
		sourceCurrency = "CNY"
	}
	targetCurrency := strings.ToUpper(strings.TrimSpace(rule.TargetCurrency))
	if targetCurrency == "" {
		targetCurrency = "USD"
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" || currency == sourceCurrency {
		return targetCurrency
	}
	return currency
}

func normalizedTextList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parseMoney(v string) float64 {
	var out float64
	fmt.Sscanf(strings.TrimSpace(v), "%f", &out)
	return out
}

func formatMoney(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
