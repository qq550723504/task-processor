package redis

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestClientPushAppendsListValues(t *testing.T) {
	client, server := newTestClientWithServer(t)
	ctx := context.Background()

	if err := client.Push(ctx, "jobs", "first"); err != nil {
		t.Fatal(err)
	}
	if err := client.Push(ctx, "jobs", "second"); err != nil {
		t.Fatal(err)
	}
	got, err := server.List("jobs")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"first", "second"}) {
		t.Fatalf("list = %v, want [first second]", got)
	}
}

func TestClientGetMissingKeyReturnsExactError(t *testing.T) {
	client := newTestClient(t)

	got, err := client.Get(context.Background(), "missing")
	if got != "" || err == nil || err.Error() != "key not found: missing" {
		t.Fatalf("Get() = %q, %v", got, err)
	}
}

func TestClientSetStoresValueWithTTL(t *testing.T) {
	client, server := newTestClientWithServer(t)

	if err := client.Set(context.Background(), "session", "active", 90*time.Second); err != nil {
		t.Fatal(err)
	}
	got, err := server.Get("session")
	if err != nil {
		t.Fatal(err)
	}
	if got != "active" {
		t.Fatalf("value = %q, want active", got)
	}
	if gotTTL := server.TTL("session"); gotTTL != 90*time.Second {
		t.Fatalf("TTL = %s, want 1m30s", gotTTL)
	}
}

func TestClientSetNXOnlyStoresFirstValue(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.SetNX(ctx, "leader", "pod-1", time.Minute)
	if err != nil || !created {
		t.Fatalf("first SetNX() = %v, %v", created, err)
	}
	created, err = client.SetNX(ctx, "leader", "pod-2", time.Minute)
	if err != nil || created {
		t.Fatalf("second SetNX() = %v, %v", created, err)
	}
	got, err := client.Get(ctx, "leader")
	if err != nil || got != "pod-1" {
		t.Fatalf("Get() = %q, %v", got, err)
	}
}

func TestClientDeleteRemovesKey(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if err := client.Set(ctx, "obsolete", "value", 0); err != nil {
		t.Fatal(err)
	}
	if err := client.Delete(ctx, "obsolete"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(ctx, "obsolete"); err == nil || err.Error() != "key not found: obsolete" {
		t.Fatalf("Get() after Delete error = %v", err)
	}
}

func TestClientScanReturnsCursorAndMatchingKeys(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	for _, key := range []string{"scan:a", "scan:b", "scan:c", "other"} {
		if err := client.Set(ctx, key, "value", 0); err != nil {
			t.Fatal(err)
		}
	}

	var (
		cursor         uint64
		got            []string
		sawNonZeroNext bool
	)
	for {
		next, keys, err := client.Scan(ctx, cursor, "scan:*", 1)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, keys...)
		if next != 0 {
			sawNonZeroNext = true
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	if !sawNonZeroNext {
		t.Fatal("Scan never returned a non-zero continuation cursor")
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"scan:a", "scan:b", "scan:c"}) {
		t.Fatalf("keys = %v", got)
	}
}

func TestClientSAddAndSMembersRoundTrip(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if err := client.SAdd(ctx, "owners", "a", "b", "a"); err != nil {
		t.Fatal(err)
	}
	got, err := client.SMembers(ctx, "owners")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("members = %v", got)
	}
}

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

func TestReplaceSetWithNoMembersDeletesSet(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if err := client.ReplaceSet(ctx, "owners", "a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := client.ReplaceSet(ctx, "owners"); err != nil {
		t.Fatal(err)
	}
	got, err := client.SMembers(ctx, "owners")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("members = %v, want empty", got)
	}
}

func TestReplaceSetUsesSingleMultiExecPipeline(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if err := client.SAdd(ctx, "owners", "stale"); err != nil {
		t.Fatal(err)
	}

	recorder := &commandRecorder{}
	client.rdb.AddHook(recorder)
	if err := client.ReplaceSet(ctx, "owners", "current"); err != nil {
		t.Fatal(err)
	}

	direct, pipelines := recorder.snapshot()
	if len(direct) != 0 {
		t.Fatalf("direct commands = %v, want none", direct)
	}
	want := [][]string{{"multi", "del", "sadd", "exec"}}
	if !slices.EqualFunc(pipelines, want, slices.Equal[[]string]) {
		t.Fatalf("pipeline commands = %v, want %v", pipelines, want)
	}
}

func TestNewRejectsNilConfig(t *testing.T) {
	client, err := New(nil)
	if client != nil || err == nil || err.Error() != "redis config is nil" {
		t.Fatalf("New(nil) = %#v, %v", client, err)
	}
}

func TestNewUsesAllConnectionConfigFields(t *testing.T) {
	server := miniredis.RunT(t)
	server.RequireAuth("secret")
	cfg := redisConfigForServer(t, server, "secret", 4, 7)

	client, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	opts := client.rdb.Options()
	if opts.Addr != server.Addr() || opts.Password != "secret" || opts.DB != 4 || opts.PoolSize != 7 {
		t.Fatalf("options = addr:%q password:%q db:%d pool:%d", opts.Addr, opts.Password, opts.DB, opts.PoolSize)
	}
	if err := client.Set(context.Background(), "configured", "yes", 0); err != nil {
		t.Fatal(err)
	}
	got, err := server.DB(4).Get("configured")
	if err != nil || got != "yes" {
		t.Fatalf("DB 4 value = %q, %v", got, err)
	}
}

func TestNewReportsConnectionFailure(t *testing.T) {
	server := miniredis.RunT(t)
	cfg := redisConfigForServer(t, server, "", 0, 1)
	server.Close()

	client, err := New(cfg)
	if client != nil || err == nil {
		t.Fatalf("New() = %#v, %v", client, err)
	}
	prefix := fmt.Sprintf("redis 连接失败 (%s:%d): ", cfg.Host, cfg.Port)
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("error = %q, want prefix %q", err, prefix)
	}
}

func TestClientOperationsPropagateBackendErrorsAfterClose(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if err := client.Set(ctx, "existing", "value", 0); err != nil {
		t.Fatal(err)
	}
	if err := client.SAdd(ctx, "existing-set", "member"); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		call           func() error
		rejectedErrors []string
	}{
		{name: "Push", call: func() error { return client.Push(ctx, "jobs", "value") }},
		{
			name: "Get existing key",
			call: func() error {
				_, err := client.Get(ctx, "existing")
				return err
			},
			rejectedErrors: []string{"key not found: existing"},
		},
		{name: "Set", call: func() error { return client.Set(ctx, "key", "value", time.Minute) }},
		{
			name: "SetNX",
			call: func() error {
				_, err := client.SetNX(ctx, "key", "value", time.Minute)
				return err
			},
		},
		{name: "Delete", call: func() error { return client.Delete(ctx, "key") }},
		{
			name: "Scan",
			call: func() error {
				_, _, err := client.Scan(ctx, 0, "*", 10)
				return err
			},
		},
		{
			name: "SMembers",
			call: func() error {
				_, err := client.SMembers(ctx, "existing-set")
				return err
			},
		},
		{name: "SAdd", call: func() error { return client.SAdd(ctx, "existing-set", "other") }},
		{name: "ReplaceSet", call: func() error { return client.ReplaceSet(ctx, "existing-set", "replacement") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("operation after Close returned nil error")
			}
			if slices.Contains(tt.rejectedErrors, err.Error()) {
				t.Fatalf("backend error was incorrectly remapped to %q", err)
			}
		})
	}
}

type commandRecorder struct {
	mu        sync.Mutex
	direct    []string
	pipelines [][]string
}

func (r *commandRecorder) DialHook(next goredis.DialHook) goredis.DialHook { return next }

func (r *commandRecorder) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		r.mu.Lock()
		r.direct = append(r.direct, cmd.Name())
		r.mu.Unlock()
		return next(ctx, cmd)
	}
}

func (r *commandRecorder) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		names := make([]string, len(cmds))
		for i, cmd := range cmds {
			names[i] = cmd.Name()
		}
		r.mu.Lock()
		r.pipelines = append(r.pipelines, names)
		r.mu.Unlock()
		return next(ctx, cmds)
	}
}

func (r *commandRecorder) snapshot() ([]string, [][]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	direct := append([]string(nil), r.direct...)
	pipelines := make([][]string, len(r.pipelines))
	for i, batch := range r.pipelines {
		pipelines[i] = append([]string(nil), batch...)
	}
	return direct, pipelines
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	client, _ := newTestClientWithServer(t)
	return client
}

func newTestClientWithServer(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	raw := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = raw.Close() })
	return &Client{rdb: raw}, server
}

func redisConfigForServer(t *testing.T, server *miniredis.Miniredis, password string, db, poolSize int) *Config {
	t.Helper()
	host, portText, err := net.SplitHostPort(server.Addr())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return &Config{Host: host, Port: port, Password: password, DB: db, PoolSize: poolSize}
}
