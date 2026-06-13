package amazonlisting

import (
	"strings"

	amazonapi "task-processor/internal/amazon/api"
)

func normalizeListingIssues(resp *amazonapi.ListingResponse) []AmazonIssue {
	if resp == nil || len(resp.Issues) == 0 {
		return nil
	}
	out := make([]AmazonIssue, 0, len(resp.Issues))
	for _, issue := range resp.Issues {
		normalized := AmazonIssue{
			Code:       strings.TrimSpace(issue.Code),
			Message:    strings.TrimSpace(issue.Message),
			Severity:   strings.TrimSpace(issue.Severity),
			IsBlocking: strings.EqualFold(strings.TrimSpace(issue.Severity), "ERROR"),
		}
		normalized.Type, normalized.Target = classifyAmazonIssue(normalized.Code, normalized.Message)
		normalized.Retryable = isRetryableAmazonIssue(normalized.Type)
		normalized.OperatorAdvice, normalized.OperatorAction = buildOperatorAdvice(normalized)
		out = append(out, normalized)
	}
	return out
}

func classifyAmazonIssue(code, message string) (issueType, target string) {
	text := strings.ToLower(strings.TrimSpace(code + " " + message))
	switch {
	case strings.Contains(text, "brand"):
		if strings.Contains(text, "missing") || strings.Contains(text, "required") {
			return "missing_brand", "brand"
		}
		return "invalid_brand", "brand"
	case strings.Contains(text, "bullet"):
		if strings.Contains(text, "missing") || strings.Contains(text, "required") {
			return "missing_bullet", "bullet_point"
		}
		return "invalid_bullet", "bullet_point"
	case strings.Contains(text, "title") || strings.Contains(text, "item_name"):
		if strings.Contains(text, "too long") || strings.Contains(text, "length") {
			return "title_too_long", "item_name"
		}
		return "invalid_title", "item_name"
	case strings.Contains(text, "image"):
		if strings.Contains(text, "main") {
			return "missing_main_image", "main_product_image_locator"
		}
		return "missing_image", "image"
	case strings.Contains(text, "price") || strings.Contains(text, "offer"):
		if strings.Contains(text, "missing") || strings.Contains(text, "required") {
			return "missing_price", "purchasable_offer"
		}
		return "invalid_price", "purchasable_offer"
	case strings.Contains(text, "sku") || strings.Contains(text, "model_number"):
		if strings.Contains(text, "missing") || strings.Contains(text, "required") {
			return "missing_sku", "sku"
		}
		return "invalid_sku", "sku"
	default:
		return "unknown", ""
	}
}

func isRetryableAmazonIssue(issueType string) bool {
	switch issueType {
	case "missing_brand", "invalid_brand",
		"missing_bullet", "invalid_bullet",
		"title_too_long", "invalid_title",
		"missing_main_image", "missing_image",
		"missing_price", "invalid_price",
		"missing_sku", "invalid_sku":
		return true
	default:
		return false
	}
}

func summarizeAmazonIssues(issues []AmazonIssue) *AmazonIssueSummary {
	if len(issues) == 0 {
		return &AmazonIssueSummary{}
	}

	summary := &AmazonIssueSummary{
		TotalCount:   len(issues),
		ActionCounts: map[string]int{},
	}
	for _, issue := range issues {
		if issue.IsBlocking {
			summary.BlockingCount++
		}
		if issue.Retryable {
			summary.RetryableCount++
			summary.RetryableIssues = append(summary.RetryableIssues, issue)
			continue
		}
		summary.ManualCount++
		summary.ManualIssues = append(summary.ManualIssues, issue)
		if advice := strings.TrimSpace(issue.OperatorAdvice); advice != "" {
			summary.ManualAdvices = append(summary.ManualAdvices, advice)
		}
		if action := strings.TrimSpace(issue.OperatorAction); action != "" {
			summary.ManualActions = append(summary.ManualActions, action)
			summary.ActionCounts[action]++
		}
	}
	summary.ManualAdvices = uniqueSorted(summary.ManualAdvices)
	summary.ManualActions = uniqueSorted(summary.ManualActions)
	return summary
}

func buildOperatorAdvice(issue AmazonIssue) (advice, action string) {
	switch issue.Type {
	case "missing_brand":
		return "补充真实品牌名，避免使用 Generic、Unknown 或占位品牌。", OperatorActionFillBrand
	case "invalid_brand":
		return "核对品牌是否为注册品牌或真实销售品牌，必要时更换为店铺可用品牌。", OperatorActionEditBrand
	case "missing_bullet":
		return "补充 3 到 5 条卖点，突出材质、尺寸、适用场景和核心功能。", OperatorActionFillBullets
	case "invalid_bullet":
		return "检查卖点是否重复、过长或包含违规表述，改成简洁可读的商品卖点。", OperatorActionEditBullets
	case "title_too_long":
		return "缩短标题，保留核心关键词、品牌、规格和品类，避免堆砌。", OperatorActionEditTitle
	case "invalid_title":
		return "检查标题是否缺少核心信息或含有违规词，按 Amazon 标题规范重写。", OperatorActionEditTitle
	case "missing_main_image":
		return "补充清晰主图，确保主体完整、白底、无水印、无拼图。", OperatorActionFillMainImage
	case "missing_image":
		return "补充缺失图片，并检查图片是否符合 Amazon 展示规范。", OperatorActionFillImages
	case "missing_price":
		return "补充售价，并确认币种、税价口径和市场站点一致。", OperatorActionFillPrice
	case "invalid_price":
		return "检查价格格式、币种和数值是否异常，必要时重新设置售价。", OperatorActionEditPrice
	case "missing_sku":
		return "补充唯一 SKU，确保不同变体不会重复。", OperatorActionFillSKU
	case "invalid_sku":
		return "检查 SKU 是否重复、过长或格式异常，按店铺规范重新生成。", OperatorActionEditSKU
	default:
		text := strings.ToLower(issue.Message)
		switch {
		case strings.Contains(text, "restricted"), strings.Contains(text, "compliance"), strings.Contains(text, "approval"):
			return "该商品可能涉及限制类目或合规审批，需要人工确认资质、证书或审核要求。", OperatorActionCheckCompliance
		case strings.Contains(text, "dangerous"), strings.Contains(text, "hazmat"):
			return "该商品可能涉及危险品规则，需要人工确认成分、运输方式和危险品资料。", OperatorActionCheckHazmat
		case strings.Contains(text, "product type"), strings.Contains(text, "browse"), strings.Contains(text, "category"):
			return "当前类目或产品类型可能不准确，需要人工重新选择 Amazon 类目和产品类型。", OperatorActionEditCategory
		case strings.Contains(text, "attribute"), strings.Contains(text, "required"):
			return "Amazon 仍缺少关键属性，建议人工补充类目必填字段后再提交。", OperatorActionFillAttributes
		default:
			return "该问题暂时无法自动修复，建议人工查看 Amazon 返回信息并补充资料。", OperatorActionManualReview
		}
	}
}
