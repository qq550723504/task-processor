//go:build integration

package productenrich_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"task-processor/internal/productenrich"
)

// =============================================================================
// LLMScoreCache 集成测试（真实 Redis）
// =============================================================================

func TestLLMScoreCache_Integration(t *testing.T) {
	suite, cleanup := setupSuite(t)
	defer cleanup()

	ctx := context.Background()
	cache := productenrich.NewLLMScoreCache(suite.redisClient, nil)

	t.Run("TextScore_SetAndGet", func(t *testing.T) {
		text := "这是一款高品质蓝牙耳机"
		require.NoError(t, cache.SetTextScore(ctx, text, 85.5, time.Minute))

		score, found := cache.GetTextScore(ctx, text)
		assert.True(t, found)
		assert.InDelta(t, 85.5, score, 0.01)
	})

	t.Run("TextScore_Miss", func(t *testing.T) {
		_, found := cache.GetTextScore(ctx, "nonexistent text xyz")
		assert.False(t, found)
	})

	t.Run("ImageScore_SetAndGet", func(t *testing.T) {
		url := "https://example.com/product.jpg"
		require.NoError(t, cache.SetImageScore(ctx, url, 72.0, time.Minute))

		score, found := cache.GetImageScore(ctx, url)
		assert.True(t, found)
		assert.InDelta(t, 72.0, score, 0.01)
	})

	t.Run("ImageScore_Miss", func(t *testing.T) {
		_, found := cache.GetImageScore(ctx, "https://example.com/notcached.jpg")
		assert.False(t, found)
	})

	t.Run("TextScore_TTL_Expiry", func(t *testing.T) {
		text := "ttl test text"
		require.NoError(t, cache.SetTextScore(ctx, text, 60.0, 100*time.Millisecond))
		time.Sleep(200 * time.Millisecond)

		_, found := cache.GetTextScore(ctx, text)
		assert.False(t, found, "score should have expired")
	})

	t.Run("Overwrite_existing_score", func(t *testing.T) {
		text := "overwrite test"
		require.NoError(t, cache.SetTextScore(ctx, text, 50.0, time.Minute))
		require.NoError(t, cache.SetTextScore(ctx, text, 90.0, time.Minute))

		score, found := cache.GetTextScore(ctx, text)
		assert.True(t, found)
		assert.InDelta(t, 90.0, score, 0.01)
	})
}

// =============================================================================
// ValidationCache 集成测试（真实 Redis）
// =============================================================================

func TestValidationCache_Integration(t *testing.T) {
	suite, cleanup := setupSuite(t)
	defer cleanup()

	ctx := context.Background()
	cache := productenrich.NewValidationCache(suite.redisClient, nil)

	t.Run("SetAndGet_valid_image", func(t *testing.T) {
		url := "https://cdn.example.com/img1.jpg"
		info := &productenrich.ImageInfo{
			URL:     url,
			IsValid: true,
			Format:  "jpeg",
			Width:   800,
			Height:  600,
		}
		require.NoError(t, cache.SetImageValidation(ctx, url, info, time.Minute))

		got, found := cache.GetImageValidation(ctx, url)
		assert.True(t, found)
		require.NotNil(t, got)
		assert.Equal(t, url, got.URL)
		assert.True(t, got.IsValid)
		assert.Equal(t, "jpeg", got.Format)
		assert.Equal(t, 800, got.Width)
		assert.Equal(t, 600, got.Height)
	})

	t.Run("SetAndGet_invalid_image", func(t *testing.T) {
		url := "https://cdn.example.com/bad.jpg"
		info := &productenrich.ImageInfo{
			URL:     url,
			IsValid: false,
			Error:   "image too small",
		}
		require.NoError(t, cache.SetImageValidation(ctx, url, info, time.Minute))

		got, found := cache.GetImageValidation(ctx, url)
		assert.True(t, found)
		require.NotNil(t, got)
		assert.False(t, got.IsValid)
		assert.Equal(t, "image too small", got.Error)
	})

	t.Run("Miss_returns_false", func(t *testing.T) {
		got, found := cache.GetImageValidation(ctx, "https://example.com/notcached.jpg")
		assert.False(t, found)
		assert.Nil(t, got)
	})

	t.Run("TTL_expiry", func(t *testing.T) {
		url := "https://cdn.example.com/ttl-test.jpg"
		info := &productenrich.ImageInfo{URL: url, IsValid: true}
		require.NoError(t, cache.SetImageValidation(ctx, url, info, 100*time.Millisecond))
		time.Sleep(200 * time.Millisecond)

		_, found := cache.GetImageValidation(ctx, url)
		assert.False(t, found, "validation should have expired")
	})
}
