package attribute

import "testing"

func TestNormalizeTranslatedNameMultis_FallbacksEmptyValues(t *testing.T) {
	processor := NewCustomAttributeProcessor()
	nameMultis := []struct {
		Language                string `json:"language"`
		AttributeValueNameMulti string `json:"attribute_value_name_multi"`
		WarningType             int    `json:"warning_type"`
	}{
		{Language: "zh-tw", AttributeValueNameMulti: "锛?白色", WarningType: 0},
		{Language: "fr", AttributeValueNameMulti: "", WarningType: 1},
		{Language: "de", AttributeValueNameMulti: "   ", WarningType: 2},
	}

	processor.normalizeTranslatedNameMultis(&nameMultis, "White")

	if got := nameMultis[0].AttributeValueNameMulti; got != ",白色" {
		t.Fatalf("expected translated comma to be fixed, got %q", got)
	}
	if got := nameMultis[1].AttributeValueNameMulti; got != "White" {
		t.Fatalf("expected empty translation to fallback to source value, got %q", got)
	}
	if got := nameMultis[2].AttributeValueNameMulti; got != "White" {
		t.Fatalf("expected blank translation to fallback to source value, got %q", got)
	}
}

func TestConvertToAttributeValueNameMultis_KeepsFallbackEntries(t *testing.T) {
	processor := NewCustomAttributeProcessor()
	source := []struct {
		Language                string `json:"language"`
		AttributeValueNameMulti string `json:"attribute_value_name_multi"`
		WarningType             int    `json:"warning_type"`
	}{
		{Language: "en", AttributeValueNameMulti: "White", WarningType: 0},
		{Language: "fr", AttributeValueNameMulti: "White", WarningType: 1},
		{Language: "", AttributeValueNameMulti: "ignored", WarningType: 0},
	}

	result := processor.convertToAttributeValueNameMultis(source)

	if len(result) != 2 {
		t.Fatalf("expected 2 valid language entries, got %d", len(result))
	}
	if result[1].Language != "fr" || result[1].AttributeValueName != "White" {
		t.Fatalf("unexpected converted fallback entry: %+v", result[1])
	}
}
