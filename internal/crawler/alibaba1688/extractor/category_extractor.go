package extractor

import (
	"encoding/json"
	"sort"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
)

// CategoryExtractor 提取1688商品类目路径。
type CategoryExtractor struct{}

func NewCategoryExtractor() *CategoryExtractor {
	return &CategoryExtractor{}
}

func (ce *CategoryExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	raw, err := page.Evaluate(`() => {
		const candidates = [];
		const seen = new Set();

		const pushCandidate = (value) => {
			if (!value) return;
			const parts = Array.isArray(value) ? value : [value];
			const cleaned = parts
				.map(part => typeof part === 'string' ? part.replace(/\s+/g, ' ').trim() : '')
				.filter(Boolean);
			if (cleaned.length < 2) return;
			const key = cleaned.join(' > ');
			if (seen.has(key)) return;
			seen.add(key);
			candidates.push(cleaned);
		};

		const collectBreadcrumbNodes = () => {
			const selectors = [
				'.breadcrumb a, .breadcrumb span',
				'[class*="breadcrumb"] a, [class*="breadcrumb"] span',
				'[class*="crumb"] a, [class*="crumb"] span',
				'[data-testid*="breadcrumb"] a, [data-testid*="breadcrumb"] span',
				'nav[aria-label*="breadcrumb" i] a, nav[aria-label*="breadcrumb" i] span'
			];
			for (const selector of selectors) {
				const nodes = Array.from(document.querySelectorAll(selector));
				const texts = nodes
					.map(node => (node.textContent || '').replace(/\s+/g, ' ').trim())
					.filter(Boolean);
				if (texts.length >= 2) {
					pushCandidate(texts);
				}
			}
		};

		const collectJsonLd = () => {
			for (const script of Array.from(document.querySelectorAll('script[type="application/ld+json"]'))) {
				try {
					const payload = JSON.parse(script.textContent || 'null');
					const items = Array.isArray(payload) ? payload : [payload];
					for (const item of items) {
						if (!item) continue;
						if (item['@type'] === 'BreadcrumbList' && Array.isArray(item.itemListElement)) {
							const texts = item.itemListElement
								.map(entry => entry?.name || entry?.item?.name || '')
								.filter(Boolean);
							pushCandidate(texts);
						}
					}
				} catch (_) {}
			}
		};

		const maybeCollectFromValue = (key, value) => {
			if (!value) return;
			const lowerKey = (key || '').toLowerCase();
			if (Array.isArray(value)) {
				if (value.length > 0 && value.every(item => typeof item === 'string')) {
					if (/(breadcrumb|crumb|category|catpath|categorypath|subject)/.test(lowerKey)) {
						pushCandidate(value);
					}
					return;
				}
				const nameList = value
					.map(item => item?.name || item?.title || item?.label || item?.text || item?.catName || item?.categoryName || '')
					.filter(Boolean);
				if (nameList.length >= 2 && /(breadcrumb|crumb|category|catpath|categorypath|subject)/.test(lowerKey)) {
					pushCandidate(nameList);
				}
			}
			if (typeof value === 'string') {
				if (/(breadcrumb|crumb|category|catpath|categorypath|subject)/.test(lowerKey) && /[>\/|]/.test(value)) {
					pushCandidate(value.split(/[>\/|]/).map(part => part.trim()).filter(Boolean));
				}
			}
		};

		const walk = (value, key = '', depth = 0, seenObjects = new WeakSet()) => {
			if (!value || depth > 6) return;
			if (typeof value !== 'object') {
				maybeCollectFromValue(key, value);
				return;
			}
			if (seenObjects.has(value)) return;
			seenObjects.add(value);
			maybeCollectFromValue(key, value);
			if (Array.isArray(value)) {
				for (const item of value) {
					walk(item, key, depth + 1, seenObjects);
				}
				return;
			}
			for (const [childKey, childValue] of Object.entries(value)) {
				maybeCollectFromValue(childKey, childValue);
				walk(childValue, childKey, depth + 1, seenObjects);
			}
		};

		collectBreadcrumbNodes();
		collectJsonLd();
		if (window.context?.result?.data) {
			walk(window.context.result.data);
		}
		if (window.__INIT_DATA?.data) {
			walk(window.__INIT_DATA.data);
		}

		return candidates;
	}`, nil)
	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取商品类目失败: %v", err)
		return err
	}

	path := selectCategoryPath(raw)
	if path == "" {
		return nil
	}
	product.Category = path
	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取到商品类目: %s", product.Category)
	return nil
}

func selectCategoryPath(raw any) string {
	candidates := decodeCategoryCandidates(raw)
	if len(candidates) == 0 {
		return ""
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := categoryPathScore(candidates[i])
		right := categoryPathScore(candidates[j])
		if left.segmentCount != right.segmentCount {
			return left.segmentCount > right.segmentCount
		}
		if left.totalLength != right.totalLength {
			return left.totalLength > right.totalLength
		}
		return candidates[i] < candidates[j]
	})
	return candidates[0]
}

type categoryScore struct {
	segmentCount int
	totalLength  int
}

func categoryPathScore(path string) categoryScore {
	parts := strings.Split(path, " > ")
	score := categoryScore{segmentCount: len(parts)}
	for _, part := range parts {
		score.totalLength += len([]rune(part))
	}
	return score
}

func decodeCategoryCandidates(raw any) []string {
	if raw == nil {
		return nil
	}

	payload, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var rows [][]string
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		path := normalizeCategoryPath(row)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func normalizeCategoryPath(parts []string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Join(strings.Fields(part), " "))
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		if lower == "首页" || lower == "home" || lower == "全部分类" || lower == "all categories" {
			continue
		}
		if _, exists := filteredSet(filtered)[part]; exists {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) < 2 {
		return ""
	}
	return strings.Join(filtered, " > ")
}

func filteredSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
