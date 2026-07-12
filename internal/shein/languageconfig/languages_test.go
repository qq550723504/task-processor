package languageconfig

import (
	"reflect"
	"testing"

	"task-processor/internal/shein/api/product"
)

func TestNormalizeFiltersDeduplicatesAndPreservesOrder(t *testing.T) {
	t.Parallel()

	got := Normalize([]product.LanguageListItem{
		{LanguageAbbr: " FR ", InputMode: 1},
		{LanguageAbbr: "en", InputMode: 1},
		{LanguageAbbr: "fr", InputMode: 2},
		{LanguageAbbr: "es", InputMode: 0},
		{LanguageAbbr: " ", InputMode: 1},
	})
	want := []string{"fr", "en"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Normalize() = %#v, want %#v", got, want)
	}
}

func TestResolveFallsBackByRegion(t *testing.T) {
	t.Parallel()

	got := Resolve([]product.LanguageListItem{{LanguageAbbr: "es", InputMode: 0}}, "JP")
	want := []string{"ja", "en"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %#v, want %#v", got, want)
	}
}
