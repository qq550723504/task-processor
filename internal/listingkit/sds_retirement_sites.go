package listingkit

import (
	"encoding/json"
	"fmt"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

func parseSDSRetirementSiteSelection(raw string) ([]sheinproduct.SubSite, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("site selection is required")
	}
	var sites []sheinproduct.SubSite
	if err := json.Unmarshal([]byte(raw), &sites); err != nil {
		return nil, fmt.Errorf("invalid site selection: %w", err)
	}
	out := make([]sheinproduct.SubSite, 0, len(sites))
	for _, site := range sites {
		abbr := strings.TrimSpace(site.SiteAbbr)
		if abbr == "" {
			return nil, fmt.Errorf("site_abbr is required")
		}
		if site.StoreType <= 0 {
			return nil, fmt.Errorf("store_type must be positive")
		}
		out = append(out, sheinproduct.SubSite{
			SiteAbbr:  abbr,
			StoreType: site.StoreType,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one site must be selected")
	}
	return out, nil
}

func buildSDSRetirementShelfRequest(item SDSRetirementItemRecord, businessModel int) (*sheinproduct.ShelfOperateRequest, error) {
	if strings.TrimSpace(item.SPUName) == "" {
		return nil, fmt.Errorf("spu_name is required")
	}
	if strings.TrimSpace(item.SKCName) == "" {
		return nil, fmt.Errorf("skc_name is required")
	}
	if businessModel <= 0 {
		return nil, fmt.Errorf("business_model must be positive")
	}
	offSites, err := parseSDSRetirementSiteSelection(item.SiteSelection)
	if err != nil {
		return nil, err
	}
	return &sheinproduct.ShelfOperateRequest{
		SpuName: item.SPUName,
		SkcSiteInfos: []sheinproduct.SkcSiteInfo{{
			BusinessModel: businessModel,
			SkcName:       item.SKCName,
			OffSubSites:   offSites,
		}},
	}, nil
}

func buildSDSRetirementDefaultSiteSelection(tasks []Task) string {
	site := ""
	for i := range tasks {
		if tasks[i].Request == nil {
			continue
		}
		// Use the same request-country defaults that normal SHEIN payload assembly
		// uses; the synced product cache currently does not expose per-SKC sites.
		for _, candidate := range defaultPlatformSites(tasks[i].Request) {
			if normalized := normalizeSDSRetirementSiteAbbr(candidate.MainSite); normalized != "" {
				site = normalized
				break
			}
			for _, subSite := range candidate.SubSites {
				if normalized := normalizeSDSRetirementSiteAbbr(subSite); normalized != "" {
					site = normalized
					break
				}
			}
			if site != "" {
				break
			}
		}
		if site != "" {
			break
		}
	}
	if site == "" {
		site = "US"
	}
	encoded, err := json.Marshal([]sheinproduct.SubSite{{SiteAbbr: site, StoreType: 1}})
	if err != nil {
		return ""
	}
	return string(encoded)
}

func normalizeSDSRetirementSiteAbbr(value string) string {
	site := strings.TrimSpace(value)
	if site == "" {
		return ""
	}
	site = strings.TrimPrefix(strings.ToLower(site), "shein-")
	return strings.ToUpper(site)
}
