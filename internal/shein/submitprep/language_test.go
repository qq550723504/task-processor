package submitprep

import "testing"

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		want        string
	}{
		{"empty defaults to english", "", "", "en"},
		{"english", "Women Summer Dress", "Casual style", "en"},
		{"chinese", "女士夏季连衣裙", "休闲风格", "zh"},
		{"japanese hiragana", "なつのドレス", "", "ja"},
		{"japanese katakana", "ナツノドレス", "", "ja"},
		{"more chinese than english", "女士夏季连衣裙 dress", "", "zh"},
		{"more english than chinese", "Women Summer Dress 连衣裙", "", "en"},
		{"more japanese than chinese", "なつのドレス 连衣裙", "", "ja"},
		{"spaces only defaults to english", "   ", "   ", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectLanguage(tt.title, tt.description); got != tt.want {
				t.Fatalf("DetectLanguage(%q, %q) = %q, want %q", tt.title, tt.description, got, tt.want)
			}
		})
	}
}

func TestLanguageHelpers(t *testing.T) {
	if !IsJapanese("なつのドレス") {
		t.Fatal("expected japanese text to be detected")
	}
	if IsJapanese("Women Dress") {
		t.Fatal("did not expect english text to be japanese")
	}
	if !IsChinese("女士连衣裙") {
		t.Fatal("expected chinese text to be detected")
	}
	if IsChinese("Women Dress") {
		t.Fatal("did not expect english text to be chinese")
	}
	if !IsEnglish("Women Summer Dress") {
		t.Fatal("expected english text to be detected")
	}
	if IsEnglish("女士连衣裙") {
		t.Fatal("did not expect chinese text to be english")
	}
	if !IsEnglish("") {
		t.Fatal("empty text should default to english")
	}
}

func TestGetCharacterCounts(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		wantJa int
		wantZh int
		wantEn int
	}{
		{"english", "Hello", 0, 0, 5},
		{"chinese", "你好世界", 0, 4, 0},
		{"japanese", "なつ", 2, 0, 0},
		{"mixed", "Hello 你好 なつ", 2, 2, 5},
		{"empty", "", 0, 0, 0},
		{"numbers and spaces ignored", "123 456", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ja, zh, en := GetCharacterCounts(tt.text)
			if ja != tt.wantJa || zh != tt.wantZh || en != tt.wantEn {
				t.Fatalf("GetCharacterCounts(%q) = (%d,%d,%d), want (%d,%d,%d)", tt.text, ja, zh, en, tt.wantJa, tt.wantZh, tt.wantEn)
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
		{"UK", "en", 1},
		{"UNKNOWN", "en", 1},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			langs := GetTargetLanguagesByRegion(tt.region)
			if len(langs) != tt.wantLen {
				t.Fatalf("len(langs) = %d, want %d for region %q", len(langs), tt.wantLen, tt.region)
			}
			if len(langs) > 0 && langs[0] != tt.wantFirst {
				t.Fatalf("langs[0] = %q, want %q for region %q", langs[0], tt.wantFirst, tt.region)
			}
		})
	}
}
