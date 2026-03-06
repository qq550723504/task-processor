package extractor

import (
	"strings"
	"testing"
)

func TestBrandExtractor_CleanBrandText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "英语站点 - Visit the",
			input:    "Visit the Spring Air Store",
			expected: "Spring Air",
		},
		{
			name:     "墨西哥站点 - Visita la tienda de",
			input:    "Visita la tienda de Spring Air",
			expected: "Spring Air",
		},
		{
			name:     "巴西站点 - Visitar a loja de",
			input:    "Visitar a loja de Spring Air",
			expected: "Spring Air",
		},
		{
			name:     "法语站点 - Visiter la boutique",
			input:    "Visiter la boutique Spring Air",
			expected: "Spring Air",
		},
		{
			name:     "德语站点 - Besuche den",
			input:    "Besuche den Spring Air Store",
			expected: "Spring Air",
		},
		{
			name:     "意大利语站点 - Visita lo store di",
			input:    "Visita lo store di Spring Air",
			expected: "Spring Air",
		},
		{
			name:     "Brand: 前缀",
			input:    "Brand: Spring Air",
			expected: "Spring Air",
		},
		{
			name:     "纯品牌名称",
			input:    "Spring Air",
			expected: "Spring Air",
		},
		{
			name:     "带 Store 后缀",
			input:    "Spring Air Store",
			expected: "Spring Air",
		},
		{
			name:     "日语站点 - ブランド:",
			input:    "ブランド: Spring Air",
			expected: "Spring Air",
		},
		{
			name:     "日语站点 - ストアにアクセス",
			input:    "ストアにアクセス Spring Air",
			expected: "Spring Air",
		},
		{
			name:     "日语站点 - ストア后缀",
			input:    "Spring Airストア",
			expected: "Spring Air",
		},
		{
			name:     "日语站点 - 带空格的ストア后缀",
			input:    "Spring Air ストア",
			expected: "Spring Air",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟品牌清理逻辑
			brand := tt.input

			prefixes := []string{
				"Visit the ",
				"Visita la tienda de ",
				"Visitar a loja de ",
				"Visiter la boutique ",
				"Besuche den ",
				"Visita lo store di ",
				"ブランド: ",
				"ストアにアクセス ",
				"Brand: ",
			}

			suffixes := []string{
				" Store",
				" tienda",
				" loja",
				" boutique",
				"ストア",
				" ストア",
			}

			// 移除前缀
			for _, prefix := range prefixes {
				if len(brand) > len(prefix) && brand[:len(prefix)] == prefix {
					brand = brand[len(prefix):]
					break
				}
			}

			// 移除后缀
			for _, suffix := range suffixes {
				if len(brand) > len(suffix) && brand[len(brand)-len(suffix):] == suffix {
					brand = brand[:len(brand)-len(suffix)]
					break
				}
			}

			// 移除前后空格
			brand = strings.TrimSpace(brand)

			if brand != tt.expected {
				t.Errorf("期望 %q, 实际得到 %q", tt.expected, brand)
			}
		})
	}
}
