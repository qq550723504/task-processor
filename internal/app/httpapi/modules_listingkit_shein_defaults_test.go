package httpapi

import "testing"

func TestResolveListingKitDefaultSheinStoreID(t *testing.T) {
	t.Parallel()

	if got := resolveListingKitDefaultSheinStoreID([]int64{869}); got != 869 {
		t.Fatalf("single store id = %d, want 869", got)
	}
	if got := resolveListingKitDefaultSheinStoreID([]int64{869, 874}); got != 0 {
		t.Fatalf("multiple store ids = %d, want 0", got)
	}
	if got := resolveListingKitDefaultSheinStoreID(nil); got != 0 {
		t.Fatalf("nil store ids = %d, want 0", got)
	}
}
