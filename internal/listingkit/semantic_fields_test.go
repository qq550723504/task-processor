package listingkit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestListingKitResultSemanticFieldNamesRemainUsable(t *testing.T) {
	result := &ListingKitResult{
		SDSDesignResult: &SDSSyncSummary{
			Status: "completed",
		},
	}
	snapshot := &StandardProductSnapshot{
		SDSDesignResult: result.SDSDesignResult,
	}

	if result.SDSDesignResult == nil || result.SDSDesignResult.Status != "completed" {
		t.Fatalf("result sds design result = %+v", result.SDSDesignResult)
	}
	if snapshot.SDSDesignResult == nil || snapshot.SDSDesignResult != result.SDSDesignResult {
		t.Fatalf("snapshot sds design result = %+v", snapshot.SDSDesignResult)
	}
}

func TestListingKitResultJSONIncludesLegacyAndSemanticSDSFields(t *testing.T) {
	result := &ListingKitResult{
		SDSSync:         &SDSSyncSummary{Status: "completed"},
		SDSDesignResult: &SDSSyncSummary{Status: "completed"},
		StandardProductSnapshot: &StandardProductSnapshot{
			SDSSync:         &SDSSyncSummary{Status: "completed"},
			SDSDesignResult: &SDSSyncSummary{Status: "completed"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	text := string(data)
	for _, key := range []string{
		`"sds_sync"`,
		`"sds_design_result"`,
	} {
		if !strings.Contains(text, key) {
			t.Fatalf("json output missing %s: %s", key, text)
		}
	}
}
