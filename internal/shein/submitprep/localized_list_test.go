package submitprep

import (
	"fmt"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

type stubTranslateAPI struct{}

func (stubTranslateAPI) Translate(text string, sourceLang string, targetLang string) (string, error) {
	return fmt.Sprintf("%s[%s->%s]", text, sourceLang, targetLang), nil
}

func TestTranslateLocalizedList_CompletesTargets(t *testing.T) {
	items := []sheinproduct.LanguageContent{
		{Language: "en", Name: "Door curtain"},
	}

	got, err := TranslateLocalizedList(items, "", []string{"en", "es"}, stubTranslateAPI{})
	if err != nil {
		t.Fatalf("TranslateLocalizedList returned error: %v", err)
	}

	if text := FindLanguageContent(got, "en"); text != "Door curtain" {
		t.Fatalf("english content = %q, want %q", text, "Door curtain")
	}
	if text := FindLanguageContent(got, "es"); text != "Door curtain[en->es]" {
		t.Fatalf("spanish content = %q, want %q", text, "Door curtain[en->es]")
	}
}

func TestTranslateLocalizedList_TranslatesMislocalizedCJK(t *testing.T) {
	items := []sheinproduct.LanguageContent{
		{Language: "en", Name: "门帘"},
	}

	got, err := TranslateLocalizedList(items, "", []string{"en", "es"}, stubTranslateAPI{})
	if err != nil {
		t.Fatalf("TranslateLocalizedList returned error: %v", err)
	}

	if text := FindLanguageContent(got, "en"); text != "门帘[zh->en]" {
		t.Fatalf("english content = %q, want %q", text, "门帘[zh->en]")
	}
	if text := FindLanguageContent(got, "es"); text != "门帘[zh->es]" {
		t.Fatalf("spanish content = %q, want %q", text, "门帘[zh->es]")
	}
}

func TestLocalizedListMissingTargets(t *testing.T) {
	items := []sheinproduct.LanguageContent{
		{Language: "en", Name: "Door curtain"},
	}

	if !LocalizedListMissingTargets(items, []string{"en", "es"}) {
		t.Fatal("expected missing target languages")
	}
	if LocalizedListMissingTargets(items, []string{"en"}) {
		t.Fatal("did not expect missing english target")
	}
}

func TestTextNeedsTranslation(t *testing.T) {
	if !TextNeedsTranslation("门帘", "en") {
		t.Fatal("expected CJK text under english label to require translation")
	}
	if TextNeedsTranslation("door curtain", "en") {
		t.Fatal("did not expect english text under english label to require translation")
	}
	if TextNeedsTranslation("门帘", "zh") {
		t.Fatal("did not expect chinese text under chinese label to require translation")
	}
}
