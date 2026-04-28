package listingkit

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {
	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	settings := s.sheinSettings
	return &settings, nil
}

func (s *service) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {
	if req == nil {
		return s.GetSheinSettings(ctx)
	}
	s.sheinSettingsMu.Lock()
	defer s.sheinSettingsMu.Unlock()
	settings := s.sheinSettings
	if req.DefaultStoreID > 0 {
		settings.DefaultStoreID = req.DefaultStoreID
	}
	if value := strings.ToUpper(strings.TrimSpace(req.Site)); value != "" {
		settings.Site = value
	}
	if value := strings.TrimSpace(req.WarehouseCode); value != "" {
		settings.WarehouseCode = value
	}
	if req.DefaultStock > 0 {
		settings.DefaultStock = req.DefaultStock
	}
	if value := strings.ToLower(strings.TrimSpace(req.DefaultSubmitMode)); value == "publish" || value == "save_draft" {
		settings.DefaultSubmitMode = value
	}
	settings.Pricing = normalizeSheinPricingRule(req.Pricing, settings.Pricing)
	now := time.Now()
	settings.UpdatedAt = &now
	s.sheinSettings = settings
	return &settings, nil
}

func (s *service) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	rule := s.currentSheinPricingRule()
	overrides := map[string]float64{}
	if task.Result.Shein.FinalDraft != nil {
		for sku, price := range task.Result.Shein.FinalDraft.ManualPriceOverrides {
			overrides[sku] = price
		}
	}
	applyToTask := false
	if req != nil {
		if req.Rule != nil {
			rule = normalizeSheinPricingRule(*req.Rule, rule)
		}
		for sku, price := range req.ManualOverrides {
			if strings.TrimSpace(sku) != "" && price > 0 {
				overrides[sku] = price
			}
		}
		applyToTask = req.ApplyToTask
	}
	review := buildSheinPricingReview(task.Result.Shein, rule, overrides)
	if applyToTask {
		applySheinPricingReview(task.Result.Shein, review)
		task.Result.UpdatedAt = time.Now()
		if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
			return nil, err
		}
	}
	return review, nil
}

func (s *service) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg := task.Result.Shein
	if pkg.FinalDraft == nil {
		pkg.FinalDraft = &sheinpub.FinalDraft{}
	}
	if req != nil {
		if req.SubmitMode != "" {
			mode := strings.ToLower(strings.TrimSpace(req.SubmitMode))
			if mode == "publish" || mode == "save_draft" {
				pkg.FinalDraft.SubmitMode = mode
			}
		}
		if len(req.ManualPriceOverrides) > 0 {
			pkg.FinalDraft.ManualPriceOverrides = clonePriceOverrides(req.ManualPriceOverrides)
		}
		if req.FinalImageOrder != nil {
			pkg.FinalDraft.FinalImageOrder = uniqueNonEmptyStrings(*req.FinalImageOrder)
		}
		if value := strings.TrimSpace(req.MainImageURL); value != "" {
			pkg.FinalDraft.MainImageURL = value
		}
		if req.DeletedImageURLs != nil {
			pkg.FinalDraft.DeletedImageURLs = uniqueNonEmptyStrings(*req.DeletedImageURLs)
		}
		if len(req.ImageRoleOverrides) > 0 {
			pkg.FinalDraft.ImageRoleOverrides = normalizeImageRoleOverrides(req.ImageRoleOverrides)
		}
		if req.Confirmed != nil {
			pkg.FinalDraft.Confirmed = *req.Confirmed
			if *req.Confirmed {
				now := time.Now()
				pkg.FinalDraft.ConfirmedAt = &now
			} else {
				pkg.FinalDraft.ConfirmedAt = nil
			}
		}
	}
	now := time.Now()
	pkg.FinalDraft.UpdatedAt = &now
	rule := s.currentSheinPricingRule()
	if pkg.Pricing != nil && pkg.Pricing.RuleSnapshot != nil {
		rule = *pkg.Pricing.RuleSnapshot
	}
	review := buildSheinPricingReview(pkg, rule, pkg.FinalDraft.ManualPriceOverrides)
	applySheinPricingReview(pkg, review)
	applySheinFinalImageDraft(pkg)
	task.Result.UpdatedAt = now
	if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
		return nil, err
	}
	return buildListingKitPreview(task, "shein")
}

func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	return &SheinSubmissionEventPage{
		TaskID: taskID,
		Items:  append([]sheinpub.SubmissionEvent(nil), task.Result.Shein.SubmissionEvents...),
	}, nil
}

func (s *service) currentSheinPricingRule() sheinpub.PricingRule {
	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	return normalizeSheinPricingRule(s.sheinSettings.Pricing, sheinpub.PricingRule{})
}

func normalizeSheinPricingRule(input sheinpub.PricingRule, fallback sheinpub.PricingRule) sheinpub.PricingRule {
	rule := fallback
	if strings.TrimSpace(rule.SourceCurrency) == "" {
		rule.SourceCurrency = "CNY"
	}
	if strings.TrimSpace(rule.TargetCurrency) == "" {
		rule.TargetCurrency = "USD"
	}
	if rule.ExchangeRate <= 0 {
		rule.ExchangeRate = 7.2
	}
	if rule.MarkupMultiplier <= 0 {
		rule.MarkupMultiplier = 2
	}
	if rule.MinimumPrice <= 0 {
		rule.MinimumPrice = 9.99
	}
	if rule.RoundTo <= 0 {
		rule.RoundTo = 0.01
	}
	if strings.TrimSpace(input.SourceCurrency) != "" {
		rule.SourceCurrency = strings.ToUpper(strings.TrimSpace(input.SourceCurrency))
	}
	if strings.TrimSpace(input.TargetCurrency) != "" {
		rule.TargetCurrency = strings.ToUpper(strings.TrimSpace(input.TargetCurrency))
	}
	if input.ExchangeRate > 0 {
		rule.ExchangeRate = input.ExchangeRate
	}
	if input.MarkupMultiplier > 0 {
		rule.MarkupMultiplier = input.MarkupMultiplier
	}
	if input.MinimumPrice > 0 {
		rule.MinimumPrice = input.MinimumPrice
	}
	if input.RoundTo > 0 {
		rule.RoundTo = input.RoundTo
	}
	if input.PriceEnding > 0 {
		rule.PriceEnding = input.PriceEnding
	}
	return rule
}

func buildSheinPricingReview(pkg *sheinpub.Package, rule sheinpub.PricingRule, overrides map[string]float64) *sheinpub.PricingReview {
	review := &sheinpub.PricingReview{
		RuleSnapshot:    &rule,
		ManualOverrides: clonePriceOverrides(overrides),
		Ready:           true,
	}
	now := time.Now()
	review.UpdatedAt = &now
	if pkg == nil || pkg.RequestDraft == nil {
		review.Ready = false
		return review
	}
	for _, skc := range pkg.RequestDraft.SKCList {
		for _, sku := range skc.SKUList {
			cost := parseMoney(sku.CostPrice)
			price := calculateSheinPrice(cost, rule)
			finalPrice := price
			manual := false
			if value, ok := overrides[sku.SupplierSKU]; ok && value > 0 {
				finalPrice = value
				manual = true
			}
			if finalPrice <= 0 {
				review.Ready = false
				review.MissingPriceSKUs = append(review.MissingPriceSKUs, sku.SupplierSKU)
			}
			review.SKUPrices = append(review.SKUPrices, sheinpub.SKUPriceReview{
				SupplierSKU:     sku.SupplierSKU,
				SupplierCode:    skc.SupplierCode,
				CostCNY:         cost,
				CalculatedPrice: price,
				FinalPrice:      finalPrice,
				Currency:        rule.TargetCurrency,
				Manual:          manual,
			})
		}
	}
	return review
}

func applySheinPricingReview(pkg *sheinpub.Package, review *sheinpub.PricingReview) {
	if pkg == nil || review == nil {
		return
	}
	pkg.Pricing = review
	priceBySKU := make(map[string]sheinpub.SKUPriceReview, len(review.SKUPrices))
	for _, price := range review.SKUPrices {
		priceBySKU[price.SupplierSKU] = price
	}
	if pkg.RequestDraft != nil {
		for skcIndex := range pkg.RequestDraft.SKCList {
			for skuIndex := range pkg.RequestDraft.SKCList[skcIndex].SKUList {
				sku := &pkg.RequestDraft.SKCList[skcIndex].SKUList[skuIndex]
				if price, ok := priceBySKU[sku.SupplierSKU]; ok && price.FinalPrice > 0 {
					value := formatMoney(price.FinalPrice)
					sku.Currency = price.Currency
					sku.BasePrice = value
					if len(sku.SitePriceList) == 0 {
						sku.SitePriceList = []sheinpub.SitePrice{{SubSite: "US"}}
					}
					for i := range sku.SitePriceList {
						sku.SitePriceList[i].BasePrice = value
						sku.SitePriceList[i].Currency = price.Currency
					}
				}
			}
		}
	}
	if pkg.PreviewProduct != nil {
		applySheinPreviewProductPrices(pkg.PreviewProduct, priceBySKU)
	}
}

func applySheinPreviewProductPrices(product *sheinproduct.Product, prices map[string]sheinpub.SKUPriceReview) {
	if product == nil {
		return
	}
	for skcIndex := range product.SKCList {
		for skuIndex := range product.SKCList[skcIndex].SKUS {
			sku := &product.SKCList[skcIndex].SKUS[skuIndex]
			price, ok := prices[sku.SupplierSKU]
			if !ok || price.FinalPrice <= 0 {
				continue
			}
			if len(sku.PriceInfoList) == 0 {
				sku.PriceInfoList = []sheinproduct.PriceInfo{{SubSite: "US"}}
			}
			for i := range sku.PriceInfoList {
				sku.PriceInfoList[i].BasePrice = price.FinalPrice
				sku.PriceInfoList[i].Currency = price.Currency
			}
			if sku.CostInfo == nil {
				sku.CostInfo = &sheinproduct.CostInfo{}
			}
			sku.CostInfo.Currency = price.Currency
		}
	}
}

func calculateSheinPrice(costCNY float64, rule sheinpub.PricingRule) float64 {
	if costCNY <= 0 || rule.ExchangeRate <= 0 {
		return 0
	}
	price := costCNY / rule.ExchangeRate * rule.MarkupMultiplier
	if price < rule.MinimumPrice {
		price = rule.MinimumPrice
	}
	if rule.PriceEnding > 0 && rule.PriceEnding < 1 {
		base := math.Floor(price)
		candidate := base + rule.PriceEnding
		if candidate < price {
			candidate = base + 1 + rule.PriceEnding
		}
		price = candidate
	}
	if rule.RoundTo > 0 {
		price = math.Ceil(price/rule.RoundTo) * rule.RoundTo
	}
	return math.Round(price*100) / 100
}

func parseMoney(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func formatMoney(value float64) string {
	return strconv.FormatFloat(math.Round(value*100)/100, 'f', 2, 64)
}

func clonePriceOverrides(input map[string]float64) map[string]float64 {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]float64, len(input))
	for sku, price := range input {
		if strings.TrimSpace(sku) != "" && price > 0 {
			out[sku] = price
		}
	}
	return out
}

func applySheinFinalImageDraft(pkg *sheinpub.Package) {
	if pkg == nil || pkg.FinalDraft == nil {
		return
	}
	order := pkg.FinalDraft.FinalImageOrder
	main := strings.TrimSpace(pkg.FinalDraft.MainImageURL)
	deleted := make(map[string]struct{}, len(pkg.FinalDraft.DeletedImageURLs))
	for _, image := range pkg.FinalDraft.DeletedImageURLs {
		deleted[strings.TrimSpace(image)] = struct{}{}
	}
	if pkg.RequestDraft != nil && pkg.RequestDraft.ImageInfo != nil {
		images := orderSheinImages(pkg.RequestDraft.ImageInfo.Gallery, order, deleted)
		if main == "" && len(images) > 0 {
			main = images[0]
		}
		if main != "" {
			pkg.RequestDraft.ImageInfo.MainImage = main
		}
		pkg.RequestDraft.ImageInfo.Gallery = images
	}
	if pkg.RequestDraft != nil {
		for i := range pkg.RequestDraft.SKCList {
			if pkg.RequestDraft.SKCList[i].ImageInfo == nil {
				continue
			}
			pkg.RequestDraft.SKCList[i].ImageInfo.Gallery = orderSheinImages(pkg.RequestDraft.SKCList[i].ImageInfo.Gallery, order, deleted)
			if _, removed := deleted[pkg.RequestDraft.SKCList[i].ImageInfo.MainImage]; removed {
				pkg.RequestDraft.SKCList[i].ImageInfo.MainImage = firstNonEmpty(pkg.RequestDraft.SKCList[i].ImageInfo.Gallery...)
			}
		}
	}
	if pkg.PreviewProduct != nil && pkg.PreviewProduct.ImageInfo != nil {
		reorderSheinProductImages(pkg.PreviewProduct.ImageInfo, order, main, deleted, pkg.FinalDraft.ImageRoleOverrides)
	}
	if pkg.PreviewProduct != nil {
		for i := range pkg.PreviewProduct.SKCList {
			reorderSheinProductImages(&pkg.PreviewProduct.SKCList[i].ImageInfo, order, main, deleted, pkg.FinalDraft.ImageRoleOverrides)
		}
	}
}

func orderSheinImages(existing []string, order []string, deleted map[string]struct{}) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(existing)+len(order))
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := deleted[value]; ok {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, image := range order {
		add(image)
	}
	for _, image := range existing {
		add(image)
	}
	return out
}

func reorderSheinProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {
	if info == nil || len(info.ImageInfoList) == 0 {
		return
	}
	priority := make(map[string]int, len(order))
	for i, image := range order {
		priority[strings.TrimSpace(image)] = i + 1
	}
	filtered := make([]sheinproduct.ImageDetail, 0, len(info.ImageInfoList))
	for _, image := range info.ImageInfoList {
		url := strings.TrimSpace(image.ImageURL)
		if url == "" {
			continue
		}
		if _, ok := deleted[url]; ok {
			continue
		}
		if url == main {
			image.ImageSort = 1
			image.MarketingMainImage = true
			image.ImageType = 1
		} else if sort, ok := priority[url]; ok {
			image.ImageSort = sort + 1
		}
		switch roles[url] {
		case "main":
			image.ImageSort = 1
			image.MarketingMainImage = true
			image.ImageType = 1
		case "swatch":
			image.ImageType = 6
			image.MarketingMainImage = false
			image.SizeImgFlag = false
		case "skc":
			image.ImageType = 2
		case "size_map":
			image.ImageType = 6
			image.SizeImgFlag = true
		}
		filtered = append(filtered, image)
	}
	info.ImageInfoList = filtered
}

func normalizeImageRoleOverrides(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for url, role := range input {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "main", "gallery", "swatch", "size_map", "skc":
			out[url] = strings.ToLower(strings.TrimSpace(role))
		}
	}
	return out
}

func appendSheinSubmissionEvent(pkg *sheinpub.Package, event sheinpub.SubmissionEvent) {
	if pkg == nil {
		return
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("%s-%d", event.Action, time.Now().UnixNano())
	}
	pkg.SubmissionEvents = append([]sheinpub.SubmissionEvent{event}, pkg.SubmissionEvents...)
	if len(pkg.SubmissionEvents) > 30 {
		pkg.SubmissionEvents = pkg.SubmissionEvents[:30]
	}
}
