package siteconfig

import (
	"strings"
	"task-processor/internal/shein/api/product"
)

func Normalize(groups []product.SiteListGroup) []product.SiteInfo {
	result := make([]product.SiteInfo, 0, len(groups))
	indexes := map[string]int{}
	seen := map[string]map[string]struct{}{}
	for _, group := range groups {
		main := strings.TrimSpace(group.MainSite)
		if main == "" {
			continue
		}
		idx, ok := indexes[main]
		if !ok {
			idx = len(result)
			indexes[main] = idx
			seen[main] = map[string]struct{}{}
			result = append(result, product.SiteInfo{MainSite: main})
		}
		for _, sub := range group.SubSiteList {
			abbr := strings.TrimSpace(sub.SiteAbbr)
			if sub.SiteStatus != 1 || abbr == "" {
				continue
			}
			if _, ok := seen[main][abbr]; ok {
				continue
			}
			seen[main][abbr] = struct{}{}
			result[idx].SubSiteList = append(result[idx].SubSiteList, abbr)
		}
	}
	filtered := result[:0]
	for _, site := range result {
		if len(site.SubSiteList) > 0 {
			site.SubSiteList = append([]string(nil), site.SubSiteList...)
			filtered = append(filtered, site)
		}
	}
	return filtered
}
