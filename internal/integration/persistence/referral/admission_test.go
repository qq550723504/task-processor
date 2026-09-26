//go:build integration

package referral

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	domain "task-processor/internal/referral"
	"testing"
	"time"
)

func TestPostgresIPLimitAcrossRepositoryInstances(t *testing.T) {
	_, db := ownedDatabase(t)
	a, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	var allowed, limited atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 20; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r := a
			if n%2 == 0 {
				r = b
			}
			err := r.AllowIP(context.Background(), "controlled-ip-hash", now)
			if err == nil {
				allowed.Add(1)
			} else if errors.Is(err, domain.ErrLimited) {
				limited.Add(1)
			} else {
				t.Errorf("unexpected limit result %v", err)
			}
		}(n)
	}
	wg.Wait()
	if allowed.Load() != 5 || limited.Load() != 15 {
		t.Fatalf("allowed=%d limited=%d", allowed.Load(), limited.Load())
	}
	if err = a.AllowIP(context.Background(), "controlled-ip-hash", now.Add(time.Minute)); err != nil {
		t.Fatalf("next window=%v", err)
	}
}
