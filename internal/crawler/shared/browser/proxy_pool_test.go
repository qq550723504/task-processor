package browser

import "testing"

func TestProxyPoolAcquireRoundRobinAndCooldown(t *testing.T) {
	pool := NewProxyPool(ProxyPoolConfig{
		Enabled:                true,
		Strategy:               "round_robin",
		FailureCooldownSeconds: 60,
		Proxies: []string{
			"http://proxy-a:8000",
			"http://proxy-b:8000",
		},
	})
	if pool == nil {
		t.Fatal("expected proxy pool to be created")
	}

	first := pool.Acquire()
	second := pool.Acquire()
	if first != "http://proxy-a:8000" {
		t.Fatalf("expected first proxy to be proxy-a, got %s", first)
	}
	if second != "http://proxy-b:8000" {
		t.Fatalf("expected second proxy to be proxy-b, got %s", second)
	}

	pool.MarkFailure(first)
	third := pool.Acquire()
	if third != "http://proxy-b:8000" {
		t.Fatalf("expected cooled-down proxy to be skipped, got %s", third)
	}
}

func TestProxyPoolSnapshotIncludesCounters(t *testing.T) {
	pool := NewProxyPool(ProxyPoolConfig{
		Enabled:                true,
		Strategy:               "round_robin",
		FailureCooldownSeconds: 60,
		Proxies: []string{
			"http://proxy-a:8000",
		},
	})
	proxy := pool.Acquire()
	pool.MarkSuccess(proxy)
	pool.MarkFailure(proxy)

	stats := pool.Snapshot()
	if stats == nil {
		t.Fatal("expected snapshot stats")
	}

	assignments, ok := stats["proxy_assignment_by_server"].(map[string]int64)
	if !ok || assignments[proxy] != 1 {
		t.Fatalf("expected assignment count for proxy, got %#v", stats["proxy_assignment_by_server"])
	}

	successes, ok := stats["proxy_success_by_server"].(map[string]int64)
	if !ok || successes[proxy] != 1 {
		t.Fatalf("expected success count for proxy, got %#v", stats["proxy_success_by_server"])
	}

	failures, ok := stats["proxy_failure_by_server"].(map[string]int64)
	if !ok || failures[proxy] != 1 {
		t.Fatalf("expected failure count for proxy, got %#v", stats["proxy_failure_by_server"])
	}
}

func TestProxyPoolAcquireHealthAwarePrefersHealthierProxy(t *testing.T) {
	pool := NewProxyPool(ProxyPoolConfig{
		Enabled:                true,
		Strategy:               "health_aware",
		FailureCooldownSeconds: 60,
		Proxies: []string{
			"http://proxy-a:8000",
			"http://proxy-b:8000",
		},
	})
	if pool == nil {
		t.Fatal("expected proxy pool to be created")
	}

	pool.MarkSuccess("http://proxy-b:8000")
	pool.MarkSuccess("http://proxy-b:8000")
	pool.MarkFailure("http://proxy-a:8000")
	pool.MarkSuccess("http://proxy-a:8000")

	got := pool.Acquire()
	if got != "http://proxy-b:8000" {
		t.Fatalf("expected health-aware strategy to prefer proxy-b, got %s", got)
	}
}
