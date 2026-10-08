package goods

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
	"unicode"
	"unicode/utf8"
)

type OfficialRuleSnapshot struct {
	ApplicationMode model.ApplicationMode   `json:"application_type"`
	Categories      []model.Category        `json:"categories"`
	Sites           []model.MainSite        `json:"sites"`
	Fill            model.FillStandards     `json:"fill"`
	Attributes      model.AttributeTemplate `json:"attributes"`
	Linked          []model.LinkedRules     `json:"linked"`
	Brands          []model.Brand           `json:"brands"`
	Warehouses      []model.Warehouse       `json:"warehouses"`
}
type OfficialDraftInput struct {
	Product model.PublishProduct `json:"product"`
	Images  []OfficialImageSlot  `json:"images"`
}
type OfficialImageSlot struct {
	Group   string `json:"group"`
	SKC     int    `json:"skc"`
	SKU     int    `json:"sku"`
	AssetID string `json:"asset_id"`
	Sort    int    `json:"sort"`
	Type    int    `json:"type"`
}

// Observations come from the image reader / committed image execution result,
// never from the browser. RemoteURL must match the exact original approval URL.
type OfficialImageObservation struct {
	AssetID      string `json:"asset_id"`
	SourceURL    string `json:"source_url"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Type         int    `json:"type"`
	RemoteURL    string `json:"remote_url,omitempty"`
	ResponseHash string `json:"response_hash,omitempty"`
	ContentHash  string `json:"content_hash"`
	Bytes        int64  `json:"bytes"`
	MediaType    string `json:"media_type"`
}
type OfficialIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type OfficialDraft struct {
	Product        model.PublishProduct `json:"product"`
	Images         []OfficialImageSlot  `json:"images"`
	Issues         []OfficialIssue      `json:"issues"`
	ReadyForUpload bool                 `json:"ready_for_upload"`
	// Only confirmed platform-image results can produce a wire payload.
	SubmissionPayload json.RawMessage `json:"submission_payload,omitempty"`
}

func BuildOfficial(input OfficialDraftInput, rules OfficialRuleSnapshot, inventory asset.ApprovedAssetInventory, observations []OfficialImageObservation) OfficialDraft {
	b := officialBuild{rules: rules, inventory: inventory}
	raw, err := json.Marshal(input)
	var copied OfficialDraftInput
	if err != nil || len(raw) > 2<<20 || json.Unmarshal(raw, &copied) != nil {
		b.issue("product", "invalid", "商品资料超过限制或包含无效值")
		return b.result
	}
	input = copied
	b.result.Product = input.Product
	b.result.Images = append([]OfficialImageSlot(nil), input.Images...)
	p := &b.result.Product
	p.SourceSystem, p.SuitFlag = "OpenAPI", "0"
	if input.Product.SuitFlag != "" && input.Product.SuitFlag != "0" {
		b.issue("suit_flag", "unsupported", "官方 API 暂不支持套装商品")
	}
	p.Sites = []model.SiteSelection{{MainSite: "shein", SubSites: []string{"shein-us"}}}
	if !rules.ApplicationMode.Valid() {
		b.issue("application_type", "rule_unavailable", "无法确认店铺绑定的官方应用类型")
	}
	if rules.ApplicationMode == model.ModeFullyManaged {
		p.Sites = nil
	}
	category, ok := findOfficialCategory(rules.Categories, p.CategoryID)
	if !ok || category.Leaf == nil || !*category.Leaf || category.ProductTypeID <= 0 {
		b.issue("category_id", "missing", "选择当前店铺可发布的末级类目")
	}
	p.ProductTypeID = category.ProductTypeID
	siteCount := 0
	for _, main := range rules.Sites {
		for _, site := range main.Sites {
			if main.ID == "shein" && site.Abbreviation == "shein-us" {
				siteCount++
				if site.Status == nil || *site.Status != 1 || site.Currency != "USD" {
					b.issue("site_list", "unavailable", "店铺美国站未启用或币种规则不可用")
				}
			}
		}
	}
	if rules.ApplicationMode != model.ModeFullyManaged && siteCount != 1 {
		b.issue("site_list", "unavailable", "无法确认当前店铺的美国站")
	}
	if rules.ApplicationMode != model.ModeFullyManaged && rules.Warehouses == nil {
		b.issue("warehouses", "rule_unavailable", "无法确认当前店铺的库存仓库")
	}
	if rules.Attributes.ProductTypeID != p.ProductTypeID || rules.Attributes.MainAttributeStatus == nil || rules.Attributes.Attributes == nil {
		b.issue("attributes", "rule_unavailable", "当前类目属性规范不可用")
	}
	for _, attribute := range rules.Attributes.Attributes {
		if attribute.Type == nil || attribute.Status == nil || attribute.Mode == nil || attribute.Show == nil || (*attribute.Type == 3 || *attribute.Type == 4) && (attribute.Dimension == nil || *attribute.Dimension != 1 && *attribute.Dimension != 3) {
			b.issue("attributes", "rule_unavailable", "当前属性层级或填写规范不完整")
		}
	}
	if rules.Fill.DefaultTitleMaximum == nil || *rules.Fill.DefaultTitleMaximum < 2 || rules.Fill.DefaultLanguage == "" {
		b.issue("multi_language_name_list", "rule_unavailable", "当前店铺默认语种和标题上限不可用")
	}
	b.languages("multi_language_name_list", p.Names, true, false)
	b.languages("multi_language_desc_list", p.Descriptions, true, true)
	if len(p.SKCs) < 1 || len(p.SKCs) > 40 {
		b.issue("skc_list", "missing", "填写 1 至 40 个 SKC")
	}
	if rules.Fill.SupplierCodeInSPU == nil {
		b.issue("supplier_code", "rule_unavailable", "当前店铺货号填写层级不可用")
	}
	if rules.Fill.SupplierCodeInSPU != nil && *rules.Fill.SupplierCodeInSPU && !officialText(p.SupplierCode, 1, 200) {
		b.issue("supplier_code", "missing", "填写 SPU 货号，最多 200 个字符")
	}
	if rules.Fill.SupplierCodeInSPU != nil && !*rules.Fill.SupplierCodeInSPU {
		p.SupplierCode = ""
	}
	if p.BrandCode == "" {
		b.issue("brand_code", "missing", "美国市场须选择当前店铺可用品牌")
	}
	if p.BrandCode != "" {
		found := false
		for _, brand := range rules.Brands {
			found = found || brand.Code == p.BrandCode
		}
		if !found {
			b.issue("brand_code", "invalid", "选择当前店铺有权使用的品牌")
		}
	}
	b.validateAttributes("product_attribute_list", p.Attributes, 3, 4, 1, 0)
	b.validateSizes(p.SizeAttributes)
	seenSKU := map[string]bool{}
	for i := range p.SKCs {
		skc := &p.SKCs[i]
		path := fmt.Sprintf("skc_list.%d", i)
		if rules.Fill.SupplierCodeInSPU != nil && !*rules.Fill.SupplierCodeInSPU && !officialText(skc.SupplierCode, 1, 200) {
			b.issue(path+".supplier_code", "missing", "填写 SKC 货号，最多 200 个字符")
		}
		if rules.Fill.SupplierCodeInSPU != nil && *rules.Fill.SupplierCodeInSPU {
			skc.SupplierCode = ""
		}
		if len(skc.Names) > 0 {
			b.languages(path+".skc_multi_language_name_list", skc.Names, true, false)
		}
		b.validateMainAttribute(path+".sale_attribute", skc.SaleAttribute, len(p.SKCs))
		if len(skc.SKUs) < 1 || len(skc.SKUs) > 400 {
			b.issue(path+".sku_list", "missing", "填写 1 至 400 个 SKU")
		}
		quantityUnit := 0
		for j := range skc.SKUs {
			sku := &skc.SKUs[j]
			sp := fmt.Sprintf("%s.sku_list.%d", path, j)
			if !officialText(sku.SupplierSKU, 1, 200) || seenSKU[sku.SupplierSKU] {
				b.issue(sp+".supplier_sku", "invalid", "填写不重复的商家 SKU，最多 200 个字符")
			}
			seenSKU[sku.SupplierSKU] = true
			for _, value := range []struct{ field, value string }{{"length", sku.Length}, {"width", sku.Width}, {"height", sku.Height}} {
				if !positiveDecimalString(value.value) {
					b.issue(sp+"."+value.field, "missing", "填写真实含包装尺寸，正数且最多两位小数")
				}
			}
			if sku.Weight == nil || !positiveMoney(*sku.Weight) {
				b.issue(sp+".weight", "missing", "填写真实含包装重量，正数且最多两位小数")
			}
			// Official default units are protocol defaults. Other units need the
			// merchant's explicit unit configuration, which cannot be inferred.
			if sku.WeightUnit != "" && sku.WeightUnit != "g" {
				b.issue(sp+".weight_unit", "rule_unavailable", "当前资料使用克；请按克填写真实重量")
			}
			if sku.DimensionUnit != "" && sku.DimensionUnit != "cm" {
				b.issue(sp+".length_width_height_unit", "rule_unavailable", "当前资料使用厘米；请按厘米填写真实尺寸")
			}
			if sku.MallState != 1 && sku.MallState != 2 {
				b.issue(sp+".mall_state", "missing", "选择在售或停售")
			}
			b.pricing(sp, *sku)
			if len(sku.Stock) < 1 || len(sku.Stock) > 100 {
				b.issue(sp+".stock_info_list", "missing", "填写真实库存")
			}
			seenWarehouse := map[string]bool{}
			for _, stock := range sku.Stock {
				if rules.ApplicationMode == model.ModeFullyManaged && (len(sku.Stock) != 1 || stock.WarehouseID != "" || stock.WarehouseName != "") {
					b.issue(sp+".stock_info_list", "invalid", "全托管填写商品可售总库存，不填写仓库")
				}
				if stock.Quantity < 0 || stock.Quantity > 99999 || seenWarehouse[stock.WarehouseID] || len(sku.Stock) > 1 && stock.WarehouseID == "" {
					b.issue(sp+".stock_info_list", "invalid", "库存范围为 0 至 99999，多仓库存须有不同的真实仓库 ID")
				}
				seenWarehouse[stock.WarehouseID] = true
				if rules.ApplicationMode != model.ModeFullyManaged {
					valid := len(rules.Warehouses) == 0 && stock.WarehouseID == "" && stock.WarehouseName == ""
					for _, warehouse := range rules.Warehouses {
						us := false
						for _, country := range warehouse.SaleCountries {
							us = us || country == "US"
						}
						if (warehouse.Code == stock.WarehouseID || len(rules.Warehouses) == 1 && stock.WarehouseID == "") && warehouse.Type == 1 && us && (stock.WarehouseName == "" || stock.WarehouseName == warehouse.Name) {
							valid = true
						}
					}
					if !valid {
						b.issue(sp+".stock_info_list", "invalid", "选择当前店铺可维护美国站库存的商家仓，多仓店铺须填写仓库 ID")
					}
				}
			}
			if sku.Quantity != nil {
				q := sku.Quantity
				if q.Type < 1 || q.Type > 3 || q.Unit < 1 || q.Unit > 3 || q.Quantity < 1 || q.Quantity > 1000000 || quantityUnit != 0 && quantityUnit != q.Unit || q.Type == 1 && q.Quantity != 1 {
					b.issue(sp+".quantity_info", "invalid", "填写有效件数；同 SKC 的件数单位须一致")
				}
				quantityUnit = q.Unit
			}
			b.validateAttributes(sp+".sale_attribute_list", sku.SaleAttributes, 1, 1, 0, skc.SaleAttribute.AttributeID)
			if len(sku.SaleAttributes) > 2 || len(sku.SaleAttributes) == 0 && len(skc.SKUs) > 1 {
				b.issue(sp+".sale_attribute_list", "invalid", "多个 SKU 须有不同的销售规格，最多两项")
			}
			b.validateAttributes(sp+".sku_scope_attribute_list", sku.Attributes, 3, 4, 3, 0)
			b.optionalSKU(sp, *sku)
		}
	}
	b.validateVariantMatrix()
	b.managedFields()
	b.validateLinkedRules()
	allRemote := b.images(input.Images, observations)
	b.fields()
	b.result.ReadyForUpload = len(b.result.Issues) == 0
	if b.result.ReadyForUpload && allRemote {
		payload, err := json.Marshal(b.result.Product)
		if err == nil && len(payload) <= 2<<20 {
			b.result.SubmissionPayload = payload
		}
	}
	return b.result
}

type officialBuild struct {
	rules     OfficialRuleSnapshot
	inventory asset.ApprovedAssetInventory
	result    OfficialDraft
}

func (b *officialBuild) issue(field, code, message string) {
	for _, issue := range b.result.Issues {
		if issue.Field == field && issue.Code == code {
			return
		}
	}
	b.result.Issues = append(b.result.Issues, OfficialIssue{field, code, message})
}
func findOfficialCategory(nodes []model.Category, id int64) (model.Category, bool) {
	for _, node := range nodes {
		if node.ID == id {
			return node, true
		}
		if found, ok := findOfficialCategory(node.Children, id); ok {
			return found, true
		}
	}
	return model.Category{}, false
}
func officialText(value string, min, max int) bool {
	if !utf8.ValidString(value) || value != strings.TrimSpace(value) || strings.ContainsAny(value, "<>") {
		return false
	}
	n := utf8.RuneCountInString(value)
	if n < min || n > max {
		return false
	}
	for _, r := range value {
		if r > 0xffff || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

var officialDecimal = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]{1,2})?$`)

func positiveDecimalString(value string) bool {
	if len(value) > 32 || !officialDecimal.MatchString(value) {
		return false
	}
	number, err := strconv.ParseFloat(value, 64)
	return err == nil && positiveMoney(number)
}
func positiveMoney(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0 && value < 1e12 && math.Abs(value*100-math.Round(value*100)) < 0.000001
}
func (b *officialBuild) languages(path string, list []model.LanguageContent, required, description bool) {
	seen := map[string]bool{}
	defaultSeen := false
	if len(list) > 256 {
		b.issue(path, "invalid", "语种数量超过限制")
		return
	}
	for _, entry := range list {
		maximum, min := 1000, 2
		if description {
			maximum, min = 5000, 1
		} else if entry.Language == b.rules.Fill.DefaultLanguage && b.rules.Fill.DefaultTitleMaximum != nil {
			maximum = *b.rules.Fill.DefaultTitleMaximum
		}
		if !description {
			for _, limit := range b.rules.Fill.TitleLimits {
				if limit.Language == entry.Language && limit.Maximum < float64(maximum) {
					maximum = int(limit.Maximum)
				}
			}
		}
		if !officialText(entry.Language, 1, 32) || seen[entry.Language] || !officialText(entry.Name, min, maximum) {
			b.issue(path+"."+entry.Language, "invalid", "内容、长度或语种不符合当前店铺规范；不支持 HTML 和表情")
		}
		seen[entry.Language] = true
		defaultSeen = defaultSeen || entry.Language == b.rules.Fill.DefaultLanguage
	}
	if required && !defaultSeen {
		b.issue(path, "missing", "填写当前店铺默认语种："+b.rules.Fill.DefaultLanguage)
	}
}
func (b *officialBuild) fields() {
	p := &b.result.Product
	p.FillConfiguration = nil
	p.FillConfigurationTags = nil
	for _, field := range b.rules.Fill.Fields {
		if field.Required == nil || field.Show == nil {
			b.issue(field.Field, "rule_unavailable", "当前字段规则不完整")
			continue
		}
		if *field.Required && !*field.Show {
			b.issue(field.Field, "rule_unavailable", "平台返回的必填和可见规则不一致")
			continue
		}
		present := false
		known := true
		switch field.Field {
		case "brand_code":
			present = p.BrandCode != ""
		case "supplier_code":
			present = p.SupplierCode != ""
			for _, skc := range p.SKCs {
				present = present || skc.SupplierCode != ""
			}
		case "skc_title":
			present = len(p.SKCs) > 0
			for _, skc := range p.SKCs {
				present = present && len(skc.Names) > 0
			}
		case "quantity_info", "package_type", "minimum_stock_quantity", "reference_product_link", "stop_purchase", "mall_state":
			present = len(p.SKCs) > 0
			for _, skc := range p.SKCs {
				for _, sku := range skc.SKUs {
					value := false
					switch field.Field {
					case "quantity_info":
						value = sku.Quantity != nil
					case "package_type":
						value = sku.PackageType != nil
					case "minimum_stock_quantity":
						number, err := strconv.Atoi(sku.MinimumStockQuantity)
						value = err == nil && number >= 1 && number <= 1000000
					case "reference_product_link":
						parsed, err := url.Parse(sku.CompetingProductLink)
						value = err == nil && parsed.Scheme == "https" && parsed.Host != "" && len(sku.CompetingProductLink) <= 300
					case "stop_purchase":
						value = sku.StopPurchase != nil && (*sku.StopPurchase == 1 || *sku.StopPurchase == 2)
					case "mall_state":
						value = sku.MallState == 1 || sku.MallState == 2
					}
					present = present && value
				}
			}
		case "shelf_require":
			present = len(p.SKCs) > 0
			for _, skc := range p.SKCs {
				present = present && (skc.ShelfRequire == "0" || skc.ShelfRequire == "1")
			}
		case "sample_spec":
			present = p.Sample != nil && len(p.SizeAttributes) > 0
		case "proof_of_stock":
			present = len(p.SKCs) > 0
			for _, skc := range p.SKCs {
				present = present && len(skc.StockProofs) == 1
			}
		case "product_detail_pic", "product_detail_picture":
			present = len(p.SKCs) > 0
			for _, skc := range p.SKCs {
				present = present && len(skc.SiteDetailImages) > 0
			}
		case "suggest_price":
			present = len(p.SKCs) > 0
			for _, skc := range p.SKCs {
				present = present && skc.SuggestedRetailPrice != nil && positiveMoney(skc.SuggestedRetailPrice.Price)
			}
		default:
			known = false
		}
		if *field.Required && *field.Show && !present {
			code, message := "missing", "补齐当前店铺要求的资料"
			if !known {
				code, message = "unsupported_required_rule", "该必填字段规范尚未支持，暂不能上传"
			}
			b.issue(field.Field, code, message)
		}
		if known && b.fieldProvided(field.Field) && !*field.Show {
			b.issue(field.Field, "forbidden", "当前店铺不允许填写此字段")
		}
	}
	quantity, packaging := false, false
	for _, skc := range p.SKCs {
		for _, sku := range skc.SKUs {
			quantity = quantity || sku.Quantity != nil
			packaging = packaging || sku.PackageType != nil
		}
	}
	if quantity {
		p.FillConfiguration = &struct {
			FilledQuantityToSKU bool `json:"filled_quantity_to_sku"`
		}{true}
		if !b.fieldShown("quantity_info") {
			b.issue("quantity_info", "forbidden", "当前类目不支持 SKU 件数填写")
		}
		for i, skc := range p.SKCs {
			for j, sku := range skc.SKUs {
				if sku.Quantity == nil {
					b.issue(fmt.Sprintf("skc_list.%d.sku_list.%d.quantity_info", i, j), "missing", "启用 SKU 件数填写时，每个 SKU 都须填写真实件数")
				}
			}
		}
	}
	if packaging {
		p.FillConfigurationTags = []string{"PACKAGE_TYPE_TO_SKU"}
		if !b.fieldShown("package_type") {
			b.issue("package_type", "forbidden", "当前类目不支持 SKU 包装填写")
		}
	}
}

func (b *officialBuild) optionalSKU(path string, sku model.ProductSKU) {
	if sku.PackageType != nil && (*sku.PackageType < "0" || *sku.PackageType > "4" || len(*sku.PackageType) != 1) {
		b.issue(path+".package_type", "invalid", "包装类型须在 0 至 4 之间")
	}
	if sku.MinimumStockQuantity != "" {
		number, err := strconv.Atoi(sku.MinimumStockQuantity)
		if err != nil || number < 1 || number > 1000000 || strconv.Itoa(number) != sku.MinimumStockQuantity {
			b.issue(path+".minimum_stock_quantity", "invalid", "最小备货数量须为 1 至 1000000 的整数")
		}
	}
	if sku.CompetingProductLink != "" {
		parsed, err := url.Parse(sku.CompetingProductLink)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || len(sku.CompetingProductLink) > 300 {
			b.issue(path+".competing_product_link", "invalid", "商品参考链接须为有效 HTTPS 链接，最多 300 字符")
		}
	}
}
func (b *officialBuild) pricing(path string, sku model.ProductSKU) {
	if b.rules.ApplicationMode == model.ModeSelfOperated {
		if len(sku.Prices) != 1 || sku.Prices[0].Currency != "USD" || sku.Prices[0].SubSite != "shein-us" || !positiveMoney(sku.Prices[0].BasePrice) {
			b.issue(path+".price_info_list", "missing", "填写美国站真实 USD 售价，正数且最多两位小数")
		}
		if sku.Cost != nil {
			b.issue(path+".cost_info", "forbidden", "自运营应用不接受托管供货价")
		}
	} else if b.rules.ApplicationMode == model.ModeSemiManaged || b.rules.ApplicationMode == model.ModeFullyManaged {
		if len(sku.Prices) > 0 {
			b.issue(path+".price_info_list", "forbidden", "托管应用由平台决定售价，请填写真实供货价")
		}
		if b.rules.Fill.Currency == nil || !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(*b.rules.Fill.Currency) {
			b.issue(path+".cost_info", "rule_unavailable", "当前店铺供货价币种不可用")
		} else if sku.Cost == nil || sku.Cost.Currency != *b.rules.Fill.Currency || !nonnegativeCost(sku.Cost.Price) {
			b.issue(path+".cost_info", "missing", "填写规范币种的真实供货价，0 至 100000 且最多两位小数")
		}
	}
	if b.rules.ApplicationMode == model.ModeFullyManaged {
		if sku.StopPurchase == nil || (*sku.StopPurchase != 1 && *sku.StopPurchase != 2) {
			b.issue(path+".stop_purchase", "missing", "全托管须选择可采或停采")
		}
	} else if sku.StopPurchase != nil {
		b.issue(path+".stop_purchase", "forbidden", "采购状态仅适用于全托管应用")
	}
}

var costPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,5})(\.[0-9]{1,2})?$`)

func nonnegativeCost(value string) bool {
	if !costPattern.MatchString(value) {
		return false
	}
	number, err := strconv.ParseFloat(value, 64)
	return err == nil && number <= 100000
}
func (b *officialBuild) fieldProvided(field string) bool {
	p := b.result.Product
	if field == "brand_code" {
		return p.BrandCode != ""
	}
	if field == "sample_spec" {
		return p.Sample != nil
	}
	if field == "supplier_code" && p.SupplierCode != "" {
		return true
	}
	for _, skc := range p.SKCs {
		switch field {
		case "supplier_code":
			if skc.SupplierCode != "" {
				return true
			}
		case "skc_title":
			if len(skc.Names) > 0 {
				return true
			}
		case "shelf_require":
			if skc.ShelfRequire != "" {
				return true
			}
		case "proof_of_stock":
			if len(skc.StockProofs) > 0 {
				return true
			}
		case "product_detail_pic", "product_detail_picture":
			if len(skc.SiteDetailImages) > 0 {
				return true
			}
		case "suggest_price":
			if skc.SuggestedRetailPrice != nil {
				return true
			}
		}
		for _, sku := range skc.SKUs {
			switch field {
			case "quantity_info":
				if sku.Quantity != nil {
					return true
				}
			case "package_type":
				if sku.PackageType != nil {
					return true
				}
			case "minimum_stock_quantity":
				if sku.MinimumStockQuantity != "" {
					return true
				}
			case "reference_product_link":
				if sku.CompetingProductLink != "" {
					return true
				}
			case "stop_purchase":
				if sku.StopPurchase != nil {
					return true
				}
			case "mall_state":
				if sku.MallState != 0 {
					return true
				}
			}
		}
	}
	return false
}
func (b *officialBuild) fieldShown(field string) bool {
	for _, rule := range b.rules.Fill.Fields {
		if rule.Field == field {
			return rule.Show != nil && *rule.Show
		}
	}
	return false
}
