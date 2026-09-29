package openai

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/knowledge"
)

// Exercises actual route admission, with no HTTP/model call. Knowledge's
// lease must cover every timeout admitted by the existing text provider seam.
func TestTextRouteTimeoutFitsKnowledgeDispatchPermitLease(t *testing.T) {
	m := textTestManager(t, "https://provider.invalid")
	for _, timeout := range []time.Duration{time.Second, 5 * time.Minute, 5*time.Minute + time.Nanosecond, knowledge.DispatchPermitLease, 6 * time.Minute} {
		m.clients["text"].config.Timeout = timeout
		_, err := m.ResolveTextRoute(context.Background(), "text")
		if timeout <= 5*time.Minute {
			if err != nil || timeout >= knowledge.DispatchPermitLease {
				t.Fatalf("valid timeout=%v lease=%v err=%v", timeout, knowledge.DispatchPermitLease, err)
			}
		} else if err == nil {
			t.Fatalf("unsafe timeout admitted: %v", timeout)
		}
	}
}
