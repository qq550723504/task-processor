package productenrich

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/pkg/hashx"
)

// --- mockRedisForCache ---

type mockRedisForCache struct {
	store  map[string]string
	getErr error
	setErr error
}

func newMockRedisForCache() *mockRedisForCache {
	return &mockRedisForCache{store: make(map[string]string)}
}

func (m *mockRedisForCache) Push(_ context.Context, key string, value string) error {
	m.store[key] = value
	return nil
}
func (m *mockRedisForCache) Get(_ context.Context, key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.store[key], nil
}
func (m *mockRedisForCache) Set(_ context.Context, key string, value string, _ time.Duration) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.store[key] = value
	return nil
}
func (m *mockRedisForCache) Delete(_ context.Context, key string) error {
	delete(m.store, key)
	return nil
}

// --- LLMScoreCache ---

func TestLLMScoreCache_NilRedis_AlwaysMiss(t *testing.T) {
	c := NewLLMScoreCache(nil, nil)

	if _, found := c.GetTextScore(context.Background(), "hello"); found {
		t.Error("expected cache miss when redisClient is nil")
	}
	if _, found := c.GetImageScore(context.Background(), "http://img.jpg"); found {
		t.Error("expected cache miss when redisClient is nil")
	}
}

func TestLLMScoreCache_NilRedis_SetReturnsNil(t *testing.T) {
	c := NewLLMScoreCache(nil, nil)

	if err := c.SetTextScore(context.Background(), "hello", 80, time.Hour); err != nil {
		t.Errorf("expected nil error for nil redis, got %v", err)
	}
	if err := c.SetImageScore(context.Background(), "http://img.jpg", 75, time.Hour); err != nil {
		t.Errorf("expected nil error for nil redis, got %v", err)
	}
}

func TestLLMScoreCache_SetAndGet_TextScore(t *testing.T) {
	rc := newMockRedisForCache()
	c := NewLLMScoreCache(rc, nil)
	ctx := context.Background()

	if err := c.SetTextScore(ctx, "product desc", 85.5, time.Hour); err != nil {
		t.Fatalf("SetTextScore: %v", err)
	}
	score, found := c.GetTextScore(ctx, "product desc")
	if !found {
		t.Fatal("expected cache hit after Set")
	}
	if score != 85.5 {
		t.Errorf("score = %.1f, want 85.5", score)
	}
}

func TestLLMScoreCache_SetAndGet_ImageScore(t *testing.T) {
	rc := newMockRedisForCache()
	c := NewLLMScoreCache(rc, nil)
	ctx := context.Background()

	if err := c.SetImageScore(ctx, "http://img.jpg", 72.0, time.Hour); err != nil {
		t.Fatalf("SetImageScore: %v", err)
	}
	score, found := c.GetImageScore(ctx, "http://img.jpg")
	if !found {
		t.Fatal("expected cache hit after Set")
	}
	if score != 72.0 {
		t.Errorf("score = %.1f, want 72.0", score)
	}
}

func TestLLMScoreCache_GetError_ReturnsMiss(t *testing.T) {
	rc := newMockRedisForCache()
	rc.getErr = errors.New("redis down")
	c := NewLLMScoreCache(rc, nil)

	_, found := c.GetTextScore(context.Background(), "text")
	if found {
		t.Error("expected miss when redis.Get returns error")
	}
}

func TestLLMScoreCache_InvalidJSON_ReturnsMiss(t *testing.T) {
	rc := newMockRedisForCache()
	rc.store["llm_score:text:"+hashx.MD5("bad")] = "not-json"
	c := NewLLMScoreCache(rc, nil)

	_, found := c.GetTextScore(context.Background(), "bad")
	if found {
		t.Error("expected miss when cached value is invalid JSON")
	}
}

func TestLLMScoreCache_DifferentKeys_NoCollision(t *testing.T) {
	rc := newMockRedisForCache()
	c := NewLLMScoreCache(rc, nil)
	ctx := context.Background()

	c.SetTextScore(ctx, "text-a", 80, time.Hour)
	c.SetTextScore(ctx, "text-b", 90, time.Hour)

	scoreA, _ := c.GetTextScore(ctx, "text-a")
	scoreB, _ := c.GetTextScore(ctx, "text-b")

	if scoreA != 80 {
		t.Errorf("scoreA = %.1f, want 80", scoreA)
	}
	if scoreB != 90 {
		t.Errorf("scoreB = %.1f, want 90", scoreB)
	}
}

// --- ValidationCache ---

func TestValidationCache_NilRedis_AlwaysMiss(t *testing.T) {
	c := NewValidationCache(nil, nil)

	if _, found := c.GetImageValidation(context.Background(), "http://img.jpg"); found {
		t.Error("expected cache miss when redisClient is nil")
	}
}

func TestValidationCache_NilRedis_SetReturnsNil(t *testing.T) {
	c := NewValidationCache(nil, nil)
	info := &ImageInfo{IsValid: true, Width: 800, Height: 600}

	if err := c.SetImageValidation(context.Background(), "http://img.jpg", info, time.Hour); err != nil {
		t.Errorf("expected nil error for nil redis, got %v", err)
	}
}

func TestValidationCache_SetAndGet_ImageValidation(t *testing.T) {
	rc := newMockRedisForCache()
	c := NewValidationCache(rc, nil)
	ctx := context.Background()

	info := &ImageInfo{IsValid: true, Width: 1024, Height: 768, Format: "jpg"}
	if err := c.SetImageValidation(ctx, "http://img.jpg", info, time.Hour); err != nil {
		t.Fatalf("SetImageValidation: %v", err)
	}

	got, found := c.GetImageValidation(ctx, "http://img.jpg")
	if !found {
		t.Fatal("expected cache hit after Set")
	}
	if !got.IsValid {
		t.Error("expected IsValid = true")
	}
	if got.Width != 1024 {
		t.Errorf("Width = %d, want 1024", got.Width)
	}
}

func TestValidationCache_GetError_ReturnsMiss(t *testing.T) {
	rc := newMockRedisForCache()
	rc.getErr = errors.New("redis timeout")
	c := NewValidationCache(rc, nil)

	_, found := c.GetImageValidation(context.Background(), "http://img.jpg")
	if found {
		t.Error("expected miss when redis.Get returns error")
	}
}

func TestValidationCache_EmptyCachedValue_ReturnsMiss(t *testing.T) {
	rc := newMockRedisForCache()
	rc.store["validation:image:"+hashx.MD5("http://img.jpg")] = ""
	c := NewValidationCache(rc, nil)

	_, found := c.GetImageValidation(context.Background(), "http://img.jpg")
	if found {
		t.Error("expected miss for empty cached value")
	}
}

func TestValidationCache_InvalidJSON_ReturnsMiss(t *testing.T) {
	rc := newMockRedisForCache()
	rc.store["validation:image:"+hashx.MD5("http://img.jpg")] = "not-json"
	c := NewValidationCache(rc, nil)

	_, found := c.GetImageValidation(context.Background(), "http://img.jpg")
	if found {
		t.Error("expected miss when cached value is invalid JSON")
	}
}
