package httpapi

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	sdsclient "task-processor/internal/sds/client"
	sdstemplate "task-processor/internal/sds/template"
)

const (
	sdsCatalogFilterPageSize = 100
	sdsCatalogMaxFilterPages = 8
	sdsCategoryPageSize      = 100
	sdsCategoryMaxPages      = 5
	sdsCatalogCacheTTL       = 10 * time.Minute
)

var sdsShipmentAreaCandidates = []struct {
	Value string
	Label string
}{
	{Value: "US", Label: "United States"},
	{Value: "CN", Label: "China"},
	{Value: "AU", Label: "Australia"},
	{Value: "CA", Label: "Canada"},
	{Value: "MX", Label: "Mexico"},
	{Value: "JP", Label: "Japan"},
	{Value: "EU", Label: "Europe"},
	{Value: "UK", Label: "United Kingdom"},
	{Value: "KR", Label: "South Korea"},
	{Value: "SG", Label: "Singapore"},
	{Value: "MY", Label: "Malaysia"},
	{Value: "PH", Label: "Philippines"},
	{Value: "TH", Label: "Thailand"},
	{Value: "VN", Label: "Vietnam"},
	{Value: "NZ", Label: "New Zealand"},
}

type sdsTemplateService interface {
	ListProducts(ctx context.Context, params sdstemplate.ListParams) (*sdstemplate.ListResponse, error)
	GetProduct(ctx context.Context, productID string) (*sdstemplate.ProductDetail, error)
}

type sdsCatalogHandler struct {
	templates sdsTemplateService
	cacheMu   sync.Mutex
	cache     map[string]sdsCatalogCacheEntry
}

type sdsCatalogCacheEntry struct {
	at   time.Time
	data any
}

type sdsCategorySummary struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type sdsShipmentAreaSummary struct {
	Value      string `json:"value"`
	Label      string `json:"label"`
	TotalCount int    `json:"totalCount"`
}

func newSDSCatalogHandler(templates sdsTemplateService) *sdsCatalogHandler {
	return &sdsCatalogHandler{
		templates: templates,
		cache:     map[string]sdsCatalogCacheEntry{},
	}
}

func (h *sdsCatalogHandler) ListSDSProducts(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	params := sdsListParamsFromQuery(c)
	weightBand := strings.TrimSpace(c.Query("weightBand"))
	cycleBand := strings.TrimSpace(c.Query("cycleBand"))
	if weightBand == "" && cycleBand == "" {
		payload, err := h.templates.ListProducts(c.Request.Context(), params)
		respondSDS(c, payload, err, "sds_product_query_failed")
		return
	}

	page := params.Page
	if page <= 0 {
		page = 1
	}
	size := params.Size
	if size <= 0 {
		size = 12
	}
	params.Page = 1
	params.Size = sdsCatalogFilterPageSize

	filtered := make([]sdstemplate.ProductSummary, 0, size)
	totalCount := 0
	fetched := 0
	for current := 1; current <= sdsCatalogMaxFilterPages; current++ {
		params.Page = current
		params.Timestamp = time.Now().UnixMilli() + int64(current)
		payload, err := h.templates.ListProducts(c.Request.Context(), params)
		if err != nil {
			respondSDS(c, nil, err, "sds_product_query_failed")
			return
		}
		if payload == nil {
			break
		}
		totalCount = max(totalCount, payload.TotalCount)
		fetched += len(payload.Items)
		for _, item := range payload.Items {
			if sdsMatchesWeightBand(item, weightBand) && sdsMatchesCycleBand(item, cycleBand) {
				filtered = append(filtered, item)
			}
		}
		if len(payload.Items) < sdsCatalogFilterPageSize || fetched >= totalCount {
			break
		}
	}

	c.JSON(http.StatusOK, &sdstemplate.ListResponse{
		Page:       page,
		Size:       size,
		TotalCount: len(filtered),
		Items:      paginateSDSProducts(filtered, page, size),
	})
}

func (h *sdsCatalogHandler) GetSDSProduct(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	payload, err := h.templates.GetProduct(c.Request.Context(), c.Param("product_id"))
	respondSDS(c, payload, err, "sds_product_detail_failed")
}

func (h *sdsCatalogHandler) ListSDSCategories(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	shipmentArea := firstNonEmptySDSCatalogValue(c.Query("shipmentArea"), "US")
	cacheKey := "categories:" + shipmentArea
	if cached, ok := h.cached(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	categoryMap := map[int64]*sdsCategorySummary{}
	totalCount := 0
	fetched := 0
	for page := 1; page <= sdsCategoryMaxPages; page++ {
		payload, err := h.templates.ListProducts(c.Request.Context(), sdstemplate.ListParams{
			Page:          page,
			Size:          sdsCategoryPageSize,
			ShipmentArea:  shipmentArea,
			PreciseSearch: "0",
			Timestamp:     time.Now().UnixMilli() + int64(page),
		})
		if err != nil {
			respondSDS(c, nil, err, "sds_category_query_failed")
			return
		}
		if payload == nil {
			break
		}
		totalCount = max(totalCount, payload.TotalCount)
		fetched += len(payload.Items)
		for _, item := range payload.Items {
			if len(item.Categories) == 0 {
				continue
			}
			leaf := item.Categories[len(item.Categories)-1]
			if leaf.ID == 0 {
				continue
			}
			existing := categoryMap[leaf.ID]
			if existing == nil {
				existing = &sdsCategorySummary{ID: leaf.ID, Name: leaf.Name}
				categoryMap[leaf.ID] = existing
			}
			existing.Count++
		}
		if len(payload.Items) < sdsCategoryPageSize || fetched >= totalCount {
			break
		}
	}

	out := make([]sdsCategorySummary, 0, len(categoryMap))
	for _, item := range categoryMap {
		out = append(out, *item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	h.setCache(cacheKey, out)
	c.JSON(http.StatusOK, out)
}

func (h *sdsCatalogHandler) ListSDSShipmentAreas(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	if cached, ok := h.cached("shipment-areas"); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	out := make([]sdsShipmentAreaSummary, 0, len(sdsShipmentAreaCandidates))
	for _, area := range sdsShipmentAreaCandidates {
		payload, err := h.templates.ListProducts(c.Request.Context(), sdstemplate.ListParams{
			Page:          1,
			Size:          1,
			ShipmentArea:  area.Value,
			PreciseSearch: "0",
			Timestamp:     time.Now().UnixMilli(),
		})
		if err != nil {
			respondSDS(c, nil, err, "sds_shipment_area_query_failed")
			return
		}
		if payload != nil && payload.TotalCount > 0 {
			out = append(out, sdsShipmentAreaSummary{
				Value:      area.Value,
				Label:      area.Label,
				TotalCount: payload.TotalCount,
			})
		}
	}
	h.setCache("shipment-areas", out)
	c.JSON(http.StatusOK, out)
}

func (h *sdsCatalogHandler) ready(c *gin.Context) bool {
	if h.templates != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "sds_catalog_unavailable",
		"message": "SDS catalog client is not configured.",
	})
	return false
}

func (h *sdsCatalogHandler) cached(key string) (any, bool) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	entry, ok := h.cache[key]
	if !ok || time.Since(entry.at) > sdsCatalogCacheTTL {
		return nil, false
	}
	return entry.data, true
}

func (h *sdsCatalogHandler) setCache(key string, data any) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	h.cache[key] = sdsCatalogCacheEntry{at: time.Now(), data: data}
}

func sdsListParamsFromQuery(c *gin.Context) sdstemplate.ListParams {
	params := sdstemplate.ListParams{
		Page:          intQuery(c, "page", 1),
		Size:          intQuery(c, "size", 12),
		ShipmentArea:  firstNonEmptySDSCatalogValue(c.Query("shipmentArea"), "US"),
		PreciseSearch: firstNonEmptySDSCatalogValue(c.Query("preciseSearch"), "0"),
		CategoryID:    strings.TrimSpace(c.Query("categoryId")),
		SortField:     strings.TrimSpace(c.Query("sortField")),
		SortType:      strings.TrimSpace(c.Query("sortType")),
		Timestamp:     time.Now().UnixMilli(),
	}
	if value := strings.TrimSpace(c.Query("keyword")); value != "" {
		params.Keyword = value
	}
	if value := intPointerQuery(c, "onSaleStatus"); value != nil {
		params.OnSaleStatus = value
	}
	if value := intPointerQuery(c, "hotSellStatus"); value != nil {
		params.HotSellStatus = value
	}
	return params
}

func respondSDS(c *gin.Context, payload any, err error, code string) {
	if err != nil {
		if isSDSHTTPAuthRequired(err) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "sds_auth_required",
				"message": "SDS 登录状态已失效，请重新登录或刷新授权后重试。",
				"detail":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": code, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, payload)
}

func isSDSHTTPAuthRequired(err error) bool {
	if err == nil {
		return false
	}
	var authErr *sdsclient.AuthRequiredError
	if errors.As(err, &authErr) {
		return true
	}
	var captchaErr *sdsclient.CaptchaRequiredError
	return errors.As(err, &captchaErr)
}

func paginateSDSProducts(items []sdstemplate.ProductSummary, page int, size int) []sdstemplate.ProductSummary {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 12
	}
	start := (page - 1) * size
	if start >= len(items) {
		return nil
	}
	end := min(start+size, len(items))
	return items[start:end]
}

func sdsMatchesWeightBand(item sdstemplate.ProductSummary, band string) bool {
	switch band {
	case "", "all":
		return true
	case "lt200":
		return sdsProductWeight(item) > 0 && sdsProductWeight(item) < 200
	case "200-500":
		return sdsProductWeight(item) >= 200 && sdsProductWeight(item) <= 500
	case "500-1000":
		return sdsProductWeight(item) > 500 && sdsProductWeight(item) <= 1000
	case "gt1000":
		return sdsProductWeight(item) > 1000
	default:
		return true
	}
}

func sdsMatchesCycleBand(item sdstemplate.ProductSummary, band string) bool {
	switch band {
	case "", "all":
		return true
	case "lt24":
		return sdsProductCycle(item) > 0 && sdsProductCycle(item) < 24
	case "24-72":
		return sdsProductCycle(item) >= 24 && sdsProductCycle(item) <= 72
	case "gt72":
		return sdsProductCycle(item) > 72
	default:
		return true
	}
}

func sdsProductWeight(item sdstemplate.ProductSummary) float64 {
	if item.Weight > 0 {
		return item.Weight
	}
	if item.MinWeight > 0 {
		return item.MinWeight
	}
	if item.WeightMin > 0 {
		return item.WeightMin
	}
	return 0
}

func sdsProductCycle(item sdstemplate.ProductSummary) int {
	if item.ProductionCycle > 0 {
		return item.ProductionCycle
	}
	if item.ProductionCycleMin > 0 {
		return item.ProductionCycleMin
	}
	if item.SmallOrderProductionCycle > 0 {
		return item.SmallOrderProductionCycle
	}
	return 0
}

func firstNonEmptySDSCatalogValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intQuery(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Query(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func intPointerQuery(c *gin.Context, key string) *int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}
