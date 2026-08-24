package openmeter

import (
	"fmt"
	"testing"
	"time"
)

func TestPoCStorageLatestSupportsIncreaseAndDecrease(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantID := pocContractTenant(fixture, "storage-increase-decrease")
	subject := mustPoCSubject(t, tenantID)

	for index, quantity := range []string{"100", "200", "50"} {
		event := mustPoCUsageEvent(t, UsageFact{
			TenantID:   tenantID,
			Metric:     MetricStorageBytesCurrent,
			Quantity:   quantity,
			SourceType: "storage_snapshot",
			SourceID:   fmt.Sprintf("poc-%s-storage-increase-decrease-%d", fixture.Environment.RunID, index),
			Revision:   fmt.Sprintf("snapshot-%d", index),
			OccurredAt: window.OccurredAt.Add(time.Duration(index) * time.Second),
		})
		mustPoCIngest(t, client, event)
		waitForPoCUsage(t, client, fixture.Meters[2].ID, subject, window.From, window.To, quantity, 1)
	}
}

func TestPoCStorageLatestUsesBusinessTimeForOutOfOrderEvents(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantID := mustPoCTenantForSubject(t, fixture.Names.SubjectB)
	subject := fixture.Names.SubjectB
	newer := mustPoCUsageEvent(t, UsageFact{
		TenantID:   tenantID,
		Metric:     MetricStorageBytesCurrent,
		Quantity:   "900",
		SourceType: "storage_snapshot",
		SourceID:   "poc-" + fixture.Environment.RunID + "-storage-out-of-order-newer",
		Revision:   "newer",
		OccurredAt: window.OccurredAt.Add(10 * time.Second),
	})
	older := mustPoCUsageEvent(t, UsageFact{
		TenantID:   tenantID,
		Metric:     MetricStorageBytesCurrent,
		Quantity:   "100",
		SourceType: "storage_snapshot",
		SourceID:   "poc-" + fixture.Environment.RunID + "-storage-out-of-order-older",
		Revision:   "older",
		OccurredAt: window.OccurredAt,
	})

	mustPoCIngest(t, client, newer)
	waitForPoCUsage(t, client, fixture.Meters[2].ID, subject, window.From, window.To, "900", 1)
	mustPoCIngest(t, client, older)
	// Visibility of the older event closes the loophole where the query could be
	// asserted before the out-of-order input was stored. The authoritative final
	// value remains the newer business-time value; do not relax this to "100".
	waitForPoCEventStored(t, fixture.SDK, older)
	waitForPoCUsage(t, client, fixture.Meters[2].ID, subject, window.From, window.To, "900", 4)
}
