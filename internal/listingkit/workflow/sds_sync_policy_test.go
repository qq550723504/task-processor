package workflow

import (
	"testing"
	"time"
)

func TestSDSDesignSyncTimeoutForVariantCount(t *testing.T) {
	t.Parallel()

	if got := SDSDesignSyncTimeoutForVariantCount(1); got != SDSDesignSyncTimeout {
		t.Fatalf("single variant timeout = %s, want %s", got, SDSDesignSyncTimeout)
	}
	if got := SDSDesignSyncTimeoutForVariantCount(3); got != SDSDesignSyncTimeout+16*5*time.Second {
		t.Fatalf("three variant timeout = %s, want %s", got, SDSDesignSyncTimeout+16*5*time.Second)
	}
	if got := SDSDesignSyncTimeoutForVariantCount(10); got != SDSDesignSyncTimeout+time.Duration(SDSDesignSyncExtraPollCap)*5*time.Second {
		t.Fatalf("capped timeout = %s, want %s", got, SDSDesignSyncTimeout+time.Duration(SDSDesignSyncExtraPollCap)*5*time.Second)
	}
}
