package amazon

import (
	"testing"
	"time"
)

func TestPageReadySettleBudget(t *testing.T) {
	if got := pageReadySettleBudget(0); got != 0 {
		t.Fatalf("timeout=0 时 settle 应为 0，实际: %v", got)
	}

	if got := pageReadySettleBudget(500 * time.Millisecond); got != pageReadyMinSettle {
		t.Fatalf("短超时时应使用最小 settle，实际: %v", got)
	}

	if got := pageReadySettleBudget(30 * time.Second); got != pageReadyMaxSettle {
		t.Fatalf("长超时时应限制到最大 settle，实际: %v", got)
	}

	if got := pageReadySettleBudget(150 * time.Millisecond); got != 150*time.Millisecond {
		t.Fatalf("超时比最小 settle 更短时不应超出 timeout，实际: %v", got)
	}
}

func TestIsLikelyRenderableBody(t *testing.T) {
	if isLikelyRenderableBody("") {
		t.Fatal("空 body 不应视为可用内容")
	}

	if isLikelyRenderableBody("  short text  ") {
		t.Fatal("过短 body 不应视为可用内容")
	}

	if !isLikelyRenderableBody("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt.") {
		t.Fatal("足够长的 body 应视为可用内容")
	}
}
