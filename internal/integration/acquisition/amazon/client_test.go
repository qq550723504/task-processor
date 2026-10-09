package amazon

import (
	"net/netip"
	"task-processor/internal/product/dataacquisition"
	"testing"
)

func TestEgressRejectsNonPublicAddressesAndNonCanonicalNavigation(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.0.1", "224.0.0.1", "fd00::1", "::ffff:127.0.0.1"} {
		if PublicAddress(netip.MustParseAddr(address)) {
			t.Fatalf("private/special address admitted: %s", address)
		}
	}
	site, _ := dataacquisition.ResolveSite("us")
	for _, address := range []string{"http://www.amazon.com/dp/B000123456", "https://www.amazon.com/cart/add", "https://www.amazon.com.evil.test/s?k=a", "https://user@www.amazon.com/s?k=a"} {
		if allowedNavigation(site, address) {
			t.Fatalf("unsafe navigation: %s", address)
		}
	}
}
