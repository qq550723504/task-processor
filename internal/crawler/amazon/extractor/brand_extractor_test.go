package extractor

import "testing"

func TestBrandSelectors(t *testing.T) {
	selectors := brandSelectors()
	expected := []string{
		"#bylineInfo",
		"a#brand",
		".po-brand .po-break-word",
		"#productBrandLogo_feature_div a",
	}

	for _, want := range expected {
		found := false
		for _, got := range selectors {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("brandSelectors() missing %q, got %v", want, selectors)
		}
	}
}

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
			if brand := normalizeBrandText(tt.input); brand != tt.expected {
				t.Errorf("期望 %q, 实际得到 %q", tt.expected, brand)
			}
		})
	}
}
