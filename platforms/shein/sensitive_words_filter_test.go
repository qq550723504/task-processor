package shein

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSensitiveWordsFilter_CheckText(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_sensitive_words.json")

	config := &SensitiveWordsConfig{
		StaticWords: map[string][]string{
			"en": {"brand", "trademark"},
			"zh": {"品牌", "商标"},
		},
		DynamicWords: map[string][]string{
			"en": {"(?i)\\b(nike|adidas)\\b", "(?i)925\\s*sterling"},
		},
		Version:  "1.0.0",
		Platform: "shein",
	}

	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)

	// 初始化过滤器
	filter, err := NewSensitiveWordsFilter(configPath)
	if err != nil {
		t.Fatalf("初始化过滤器失败: %v", err)
	}

	tests := []struct {
		name     string
		text     string
		language string
		want     bool
	}{
		{
			name:     "硬编码敏感词 - 925 Sterling",
			text:     "This is a 925 Sterling Silver ring",
			language: "en",
			want:     true,
		},
		{
			name:     "硬编码敏感词 - 925Sterling 无空格",
			text:     "925Sterling silver jewelry",
			language: "en",
			want:     true,
		},
		{
			name:     "配置文件静态敏感词",
			text:     "This is a brand new product",
			language: "en",
			want:     true,
		},
		{
			name:     "配置文件动态敏感词",
			text:     "Nike shoes are great",
			language: "en",
			want:     true,
		},
		{
			name:     "中文敏感词",
			text:     "这是一个品牌产品",
			language: "zh",
			want:     true,
		},
		{
			name:     "无敏感词",
			text:     "This is a normal product description",
			language: "en",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, words := filter.CheckText(tt.text, tt.language)
			if got != tt.want {
				t.Errorf("CheckText() = %v, want %v, found words: %v", got, tt.want, words)
			}
			if got && len(words) == 0 {
				t.Error("CheckText() 返回 true 但未返回敏感词列表")
			}
		})
	}
}

func TestSensitiveWordsFilter_CheckProduct(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_sensitive_words.json")

	config := &SensitiveWordsConfig{
		StaticWords: map[string][]string{
			"en": {"brand"},
		},
		DynamicWords: map[string][]string{
			"en": {"(?i)925\\s*sterling"},
		},
		Version:  "1.0.0",
		Platform: "shein",
	}

	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)

	filter, err := NewSensitiveWordsFilter(configPath)
	if err != nil {
		t.Fatalf("初始化过滤器失败: %v", err)
	}

	tests := []struct {
		name        string
		title       string
		description string
		languages   []string
		want        bool
	}{
		{
			name:        "标题包含敏感词",
			title:       "925 Sterling Silver Ring",
			description: "Beautiful jewelry",
			languages:   []string{"en"},
			want:        true,
		},
		{
			name:        "描述包含敏感词",
			title:       "Silver Ring",
			description: "This is a brand new product",
			languages:   []string{"en"},
			want:        true,
		},
		{
			name:        "无敏感词",
			title:       "Silver Ring",
			description: "Beautiful jewelry",
			languages:   []string{"en"},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, foundWords := filter.CheckProduct(tt.title, tt.description, tt.languages)
			if got != tt.want {
				t.Errorf("CheckProduct() = %v, want %v, found words: %v", got, tt.want, foundWords)
			}
		})
	}
}
