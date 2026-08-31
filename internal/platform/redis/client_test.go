package redis

import (
	"context"
	"slices"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestReplaceSetRemovesStaleMembers(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if err := client.ReplaceSet(ctx, "owners", "a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := client.ReplaceSet(ctx, "owners", "b", "c"); err != nil {
		t.Fatal(err)
	}
	got, err := client.SMembers(ctx, "owners")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("members = %v", got)
	}
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	server := miniredis.RunT(t)
	raw := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = raw.Close() })
	return &Client{rdb: raw}
}
