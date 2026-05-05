package listingkit

import (
	"testing"
	"time"
)

func TestSDSDesignSyncTimeoutForVariantCount(t *testing.T) {
	t.Parallel()

	if got := sdsDesignSyncTimeoutForVariantCount(1); got != sdsDesignSyncTimeout {
		t.Fatalf("single variant timeout = %s, want %s", got, sdsDesignSyncTimeout)
	}
	if got := sdsDesignSyncTimeoutForVariantCount(3); got != sdsDesignSyncTimeout+16*5*time.Second {
		t.Fatalf("three variant timeout = %s, want %s", got, sdsDesignSyncTimeout+16*5*time.Second)
	}
	if got := sdsDesignSyncTimeoutForVariantCount(10); got != sdsDesignSyncTimeout+time.Duration(sdsDesignSyncExtraPollCap)*5*time.Second {
		t.Fatalf("capped timeout = %s, want %s", got, sdsDesignSyncTimeout+time.Duration(sdsDesignSyncExtraPollCap)*5*time.Second)
	}
}
