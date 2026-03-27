package translate

import (
	"testing"
)

func TestLanguageDetector_DetectLanguage(t *testing.T) {
	d := NewLanguageDetector()

	tests := []struct {
		name        string
		title       string
		description string
		want        string
	}{
		{"空文本默认英文", "", "", "en"},
		{"纯英文", "Women Summer Dress", "Casual style", "en"},
		{"纯中文", "女士夏季连衣裙", "休闲风格", "zh"},
		{"纯日文平假名", "なつのドレス", "", "ja"},
		{"纯日文片假名", "ナツノドレス", "", "ja"},
		{"中文多于英文", "女士夏季连衣裙 dress", "", "zh"},
		{"英文多于中文", "Women Summer Dress 连衣裙", "", "en"},
		{"日文多于中文", "なつのドレス 连衣裙", "", "ja"},
		{"仅空格默认英文", "   ", "   ", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.DetectLanguage(tt.title, tt.description)
			if got != tt.want {
				t.Errorf("DetectLanguage(%q, %q) = %q, want %q", tt.title, tt.description, got, tt.want)
			}
		})
	}
}

func TestLanguageDetector_IsJapanese(t *testing.T) {
	d := NewLanguageDetector()

	tests := []struct {
		text string
		want bool
	}{
		{"なつのドレス", true},
		{"ナツノドレス", true},
		{"Women Dress", false},
		{"女士连衣裙", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := d.IsJapanese(tt.text); got != tt.want {
				t.Errorf("IsJapanese(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestLanguageDetector_IsChinese(t *testing.T) {
	d := NewLanguageDetector()

	tests := []struct {
		text string
		want bool
	}{
		{"女士连衣裙", true},
		{"Women Dress", false},
		{"なつのドレス", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := d.IsChinese(tt.text); got != tt.want {
				t.Errorf("IsChinese(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestLanguageDetector_IsEnglish(t *testing.T) {
	d := NewLanguageDetector()

	tests := []struct {
		text string
		want bool
	}{
		{"Women Summer Dress", true},
		{"女士连衣裙", false},
		{"なつのドレス", false},
		{"", true}, // 空文本默认英文
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := d.IsEnglish(tt.text); got != tt.want {
				t.Errorf("IsEnglish(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestLanguageDetector_GetCharacterCounts(t *testing.T) {
	d := NewLanguageDetector()

	tests := []struct {
		name   string
		text   string
		wantJa int
		wantZh int
		wantEn int
	}{
		{"纯英文", "Hello", 0, 0, 5},
		{"纯中文", "你好世界", 0, 4, 0},
		{"纯日文平假名", "なつ", 2, 0, 0},
		{"混合", "Hello 你好 なつ", 2, 2, 5},
		{"空字符串", "", 0, 0, 0},
		{"数字和空格不计入", "123 456", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ja, zh, en := d.GetCharacterCounts(tt.text)
			if ja != tt.wantJa {
				t.Errorf("japanese = %d, want %d", ja, tt.wantJa)
			}
			if zh != tt.wantZh {
				t.Errorf("chinese = %d, want %d", zh, tt.wantZh)
			}
			if en != tt.wantEn {
				t.Errorf("english = %d, want %d", en, tt.wantEn)
			}
		})
	}
}

func TestGetTargetLanguagesByRegion(t *testing.T) {
	tests := []struct {
		region    string
		wantFirst string
		wantLen   int
	}{
		{"US", "en", 2},
		{"MX", "en", 2},
		{"FR", "de", 5},
		{"DE", "de", 5},
		{"IT", "de", 5},
		{"ES", "de", 5},
		{"JP", "ja", 2},
		{"SA", "ar", 2},
		{"AE", "ar", 2},
		{"UK", "en", 1},      // default
		{"UNKNOWN", "en", 1}, // default
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			langs := GetTargetLanguagesByRegion(tt.region)
			if len(langs) != tt.wantLen {
				t.Errorf("len(langs) = %d, want %d for region %q", len(langs), tt.wantLen, tt.region)
			}
			if len(langs) > 0 && langs[0] != tt.wantFirst {
				t.Errorf("langs[0] = %q, want %q for region %q", langs[0], tt.wantFirst, tt.region)
			}
		})
	}
}
