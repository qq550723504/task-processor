package httpapi

import "testing"

func TestResolveListingKitDefaultSheinStoreID(t *testing.T) {
	t.Parallel()

	if got := resolveListingKitDefaultSheinStoreID([]int64{873}); got != 873 {
		t.Fatalf("single store id = %d, want 873", got)
	}
	if got := resolveListingKitDefaultSheinStoreID([]int64{873, 874}); got != 0 {
		t.Fatalf("multiple store ids = %d, want 0", got)
	}
	if got := resolveListingKitDefaultSheinStoreID(nil); got != 0 {
		t.Fatalf("nil store ids = %d, want 0", got)
	}
}
