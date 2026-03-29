// Package worker provides unit tests for worker pool
package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerJob tests worker job structure
func TestWorkerJob(t *testing.T) {
	job := WorkerJob{
		TaskID:   12345,
		TenantID: "tenant-1",
		ShopID:  "shop-837",
		TaskData: `{"product_id": "abc123"}`,
	}

	assert.Equal(t, int64(12345), job.TaskID)
	assert.Equal(t, "tenant-1", job.TenantID)
	assert.Equal(t, "shop-837", job.ShopID)
	assert.Equal(t, `{"product_id": "abc123"}`, job.TaskData)
}

// TestWorkerJob_GetID tests GetID method
func TestWorkerJob_GetID(t *testing.T) {
	job := WorkerJob{TaskID: 999}
	assert.Equal(t, int64(999), job.GetID())
}

// TestQueueStats tests queue statistics
func TestQueueStats(t *testing.T) {
	stats := QueueStats{
		QueueSize:      50,
		BufferSize:     100,
		AvailableSlots: 50,
		UsagePercent:   50.0,
	}

	assert.Equal(t, 50, stats.QueueSize)
	assert.Equal(t, 100, stats.BufferSize)
	assert.Equal(t, 50, stats.AvailableSlots)
	assert.Equal(t, 50.0, stats.UsagePercent)
}

// TestPoolConfig_Default tests default pool configuration
func TestPoolConfig_Default(t *testing.T) {
	config := DefaultPoolConfig()

	assert.Equal(t, 5, config.Concurrency)
	assert.Equal(t, 100, config.BufferSize)
	assert.Equal(t, 15*time.Minute, config.TaskTimeout)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, 30*time.Second, config.ShutdownTimeout)
}

// TestPoolConfig_Validate tests configuration validation
func TestPoolConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      PoolConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: PoolConfig{
				Concurrency:     5,
				BufferSize:      100,
				TaskTimeout:     15 * time.Minute,
				ShutdownTimeout: 30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "invalid concurrency",
			config: PoolConfig{
				Concurrency:     0,
				BufferSize:      100,
				TaskTimeout:     15 * time.Minute,
				ShutdownTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "invalid buffer size",
			config: PoolConfig{
				Concurrency:     5,
				BufferSize:      0,
				TaskTimeout:     15 * time.Minute,
				ShutdownTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "invalid task timeout",
			config: PoolConfig{
				Concurrency:     5,
				BufferSize:      100,
				TaskTimeout:     0,
				ShutdownTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "invalid shutdown timeout",
			config: PoolConfig{
				Concurrency:     5,
				BufferSize:      100,
				TaskTimeout:     15 * time.Minute,
				ShutdownTimeout: 0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPoolConfig_ValidateCorrections tests configuration auto-correction
func TestPoolConfig_ValidateCorrections(t *testing.T) {
	config := PoolConfig{
		Concurrency:     -5,  // invalid
		BufferSize:      -10,  // invalid
		TaskTimeout:     -1,   // invalid
		ShutdownTimeout: -30,   // invalid
	}

	err := config.Validate()
	assert.Error(t, err)

	// Check corrections
	assert.Equal(t, 1, config.Concurrency)
	assert.Equal(t, 10, config.BufferSize)
	assert.Equal(t, 15*time.Minute, config.TaskTimeout)
	assert.Equal(t, 30*time.Second, config.ShutdownTimeout)
}
