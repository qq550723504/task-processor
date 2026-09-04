package listingkit

import (
	"reflect"
	"testing"
)

func TestAIChatCompleterExposesOnlyTextChatCapabilities(t *testing.T) {
	t.Parallel()

	contract := reflect.TypeOf((*AIChatCompleter)(nil)).Elem()
	if _, exists := contract.MethodByName("AnalyzeImage"); exists {
		t.Fatal("AIChatCompleter still exposes retired ListingKit image analysis")
	}
	for _, method := range []string{"CreateChatCompletion", "Generate", "GetDefaultModel"} {
		if _, exists := contract.MethodByName(method); !exists {
			t.Fatalf("AIChatCompleter is missing text capability %q", method)
		}
	}
}
