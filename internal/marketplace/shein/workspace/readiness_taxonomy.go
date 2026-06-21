package workspace

func BuildReadinessTaxonomy(key string, warningOnly bool) ReadinessTaxonomy {
	severity := "blocker"
	if warningOnly {
		severity = "warning"
	}
	taxonomy := ReadinessTaxonomy{
		BlockerKey:   "unknown",
		Severity:     severity,
		Domain:       "system",
		RepairTarget: "integration_gap",
		RepairRoute:  "workspace.readiness_detail",
	}
	switch key {
	case "shein_cookie_unavailable":
		taxonomy.BlockerKey = "shein_store_auth_unavailable"
		taxonomy.Domain = "store"
		taxonomy.RepairTarget = "store_login"
		taxonomy.RepairRoute = "settings.shein_store_login"
		taxonomy.Recoverable = true
	case "pod_platform":
		taxonomy.BlockerKey = "pod_platform_not_ready"
		taxonomy.Domain = "system"
		taxonomy.RepairTarget = "pod_platform"
		taxonomy.RepairRoute = "workspace.pod_execution"
		taxonomy.Recoverable = true
	case "category", "category_review":
		taxonomy.BlockerKey = "missing_category"
		taxonomy.Domain = "category"
		taxonomy.RepairTarget = "category_review"
		taxonomy.RepairRoute = "workspace.category"
		taxonomy.Recoverable = true
	case "attributes", "attribute_review":
		taxonomy.BlockerKey = "missing_required_attribute"
		taxonomy.Domain = "attribute"
		taxonomy.RepairTarget = "attribute_review"
		taxonomy.RepairRoute = "workspace.attributes"
		taxonomy.Recoverable = true
	case "sale_attributes":
		taxonomy.BlockerKey = "missing_sale_attribute"
		taxonomy.Domain = "sale_attribute"
		taxonomy.RepairTarget = "sale_attribute_review"
		taxonomy.RepairRoute = "workspace.sale_attributes"
		taxonomy.Recoverable = true
	case "images", "final_images", "variant_image_coverage":
		taxonomy.BlockerKey = "image_upload_failed"
		taxonomy.Domain = "image"
		taxonomy.RepairTarget = "image_review"
		taxonomy.RepairRoute = "workspace.images"
		taxonomy.Recoverable = true
	case "variants":
		taxonomy.BlockerKey = "sku_invalid"
		taxonomy.Domain = "sku"
		taxonomy.RepairTarget = "sku_review"
		taxonomy.RepairRoute = "workspace.variants"
		taxonomy.Recoverable = true
	case "pricing":
		taxonomy.BlockerKey = "price_invalid"
		taxonomy.Domain = "price"
		taxonomy.RepairTarget = "pricing_review"
		taxonomy.RepairRoute = "workspace.pricing"
		taxonomy.Recoverable = true
	case "request_draft", "preview_product", "final_review", "source_facts", "manual_notes":
		taxonomy.BlockerKey = key
		taxonomy.Domain = "system"
		taxonomy.RepairTarget = key
		taxonomy.RepairRoute = "workspace.review"
		taxonomy.Recoverable = true
	default:
		taxonomy.RequiresEngineering = true
	}
	return taxonomy
}
