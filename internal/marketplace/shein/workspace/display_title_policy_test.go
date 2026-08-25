package workspace

import "testing"

func TestResolveDisplayTitlePrefersProductTitle(t *testing.T) {
	got := ResolveDisplayTitle(DisplayTitleInput{
		ProductNameEn:    "  English title  ",
		ProductNameMulti: "多语言标题",
		SpuName:          "SPU title",
		TitleSource:      "unresolved_prompt_title",
	})
	if got != "  English title  " {
		t.Fatalf("display title = %q, want original product title", got)
	}
}

func TestResolveDisplayTitleFallsBackToSPU(t *testing.T) {
	got := ResolveDisplayTitle(DisplayTitleInput{
		ProductNameMulti: "   ",
		SpuName:          "  SPU title  ",
		TitleSource:      "manual",
	})
	if got != "SPU title" {
		t.Fatalf("display title = %q, want trimmed SPU title", got)
	}
}

func TestResolveDisplayTitleSuppressesUnsafeFallback(t *testing.T) {
	got := ResolveDisplayTitle(DisplayTitleInput{
		SpuName:       "SPU title",
		TitleSource:   "  structured_fallback  ",
		ProductNameEn: "  ",
	})
	if got != "" {
		t.Fatalf("display title = %q, want empty title for unsafe fallback source", got)
	}
}
