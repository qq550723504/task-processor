package listingkit

import "testing"

func TestRestoreDetailContextAndSafetyValueHandleNil(t *testing.T) {
	t.Parallel()

	if restoreDetailContextValue(nil) != nil {
		t.Fatal("restoreDetailContextValue(nil) should return nil")
	}
	if restoreDetailSafetyValue(nil) != nil {
		t.Fatal("restoreDetailSafetyValue(nil) should return nil")
	}
}
