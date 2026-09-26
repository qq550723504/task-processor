package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	domain "task-processor/internal/accountallocation"
)

func TestAllocationResponsesMatchSeparateSnapshotAndMutationContracts(t *testing.T) {
	allocation := domain.Allocation{
		OrganizationID: "org-1", Metric: domain.MetricToken,
		WindowStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Allocated:   10, Consumed: 2, Remaining: 8, Version: 1, Active: true,
	}
	data, err := json.Marshal(allocationResponseFrom(allocation))
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if _, ok := value["organizationId"]; ok {
		t.Fatalf("member allocation leaked snapshot scope field: %s", data)
	}
	for _, key := range []string{"metric", "windowStart", "windowEnd", "allocated", "consumed", "remaining", "version", "active"} {
		if _, ok := value[key]; !ok {
			t.Fatalf("member allocation omitted %q: %s", key, data)
		}
	}
}
