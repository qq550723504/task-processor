package workspace

import "testing"

func TestBuildReadinessTaxonomyForKnownKeys(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		key          string
		blockerKey   string
		domain       string
		repairTarget string
		repairRoute  string
	}{
		"category": {
			key:          "category",
			blockerKey:   "missing_category",
			domain:       "category",
			repairTarget: "category_review",
			repairRoute:  "workspace.category",
		},
		"attribute": {
			key:          "attributes",
			blockerKey:   "missing_required_attribute",
			domain:       "attribute",
			repairTarget: "attribute_review",
			repairRoute:  "workspace.attributes",
		},
		"sale_attribute": {
			key:          "sale_attributes",
			blockerKey:   "missing_sale_attribute",
			domain:       "sale_attribute",
			repairTarget: "sale_attribute_review",
			repairRoute:  "workspace.sale_attributes",
		},
		"image": {
			key:          "images",
			blockerKey:   "image_upload_failed",
			domain:       "image",
			repairTarget: "image_review",
			repairRoute:  "workspace.images",
		},
		"price": {
			key:          "pricing",
			blockerKey:   "price_invalid",
			domain:       "price",
			repairTarget: "pricing_review",
			repairRoute:  "workspace.pricing",
		},
		"sku": {
			key:          "variants",
			blockerKey:   "sku_invalid",
			domain:       "sku",
			repairTarget: "sku_review",
			repairRoute:  "workspace.variants",
		},
		"store": {
			key:          "shein_cookie_unavailable",
			blockerKey:   "shein_store_auth_unavailable",
			domain:       "store",
			repairTarget: "store_login",
			repairRoute:  "settings.shein_store_login",
		},
	}

	for name, tc := range cases {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := BuildReadinessTaxonomy(tc.key, false)
			if got.BlockerKey != tc.blockerKey || got.Domain != tc.domain || got.RepairTarget != tc.repairTarget || got.RepairRoute != tc.repairRoute {
				t.Fatalf("taxonomy = %+v, want %s/%s/%s/%s", got, tc.blockerKey, tc.domain, tc.repairTarget, tc.repairRoute)
			}
			if got.Severity != "blocker" || !got.Recoverable || got.RequiresEngineering {
				t.Fatalf("taxonomy severity/recovery = %+v, want blocker/recoverable/user-repair", got)
			}
		})
	}
}

func TestBuildReadinessTaxonomyMarksWarningsAndUnknownKeys(t *testing.T) {
	t.Parallel()

	warning := BuildReadinessTaxonomy("manual_notes", true)
	if warning.Severity != "warning" || warning.Domain != "system" || !warning.Recoverable {
		t.Fatalf("warning taxonomy = %+v, want warning/system/recoverable", warning)
	}

	unknown := BuildReadinessTaxonomy("remote_compliance_hold", false)
	if unknown.BlockerKey != "unknown" || unknown.Domain != "system" || unknown.RepairTarget != "integration_gap" {
		t.Fatalf("unknown taxonomy = %+v, want unknown/system/integration_gap", unknown)
	}
	if unknown.Recoverable || !unknown.RequiresEngineering {
		t.Fatalf("unknown recovery = %+v, want non-recoverable engineering escalation", unknown)
	}
}
