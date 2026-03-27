package distributed

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPendingRegistry_RegisterAndDeliver(t *testing.T) {
	reg := NewPendingRegistry(5 * time.Second)

	pt := reg.Register(context.Background(), "123")
	assert.Equal(t, "123", pt.TaskID)
	assert.Equal(t, 1, reg.Len())

	result := &CrawlResult{TaskID: "123", Success: true}
	found := reg.Deliver(result)
	assert.True(t, found)
	assert.Equal(t, 0, reg.Len())

	got, err := reg.Wait(pt)
	require.NoError(t, err)
	assert.True(t, got.Success)
}

func TestPendingRegistry_Timeout(t *testing.T) {
	reg := NewPendingRegistry(50 * time.Millisecond)

	pt := reg.Register(context.Background(), "456")
	_, err := reg.Wait(pt)
	assert.ErrorContains(t, err, "爬虫任务超时")
	assert.Equal(t, 0, reg.Len())
}

func TestPendingRegistry_DeliverUnknownTask(t *testing.T) {
	reg := NewPendingRegistry(5 * time.Second)
	found := reg.Deliver(&CrawlResult{TaskID: "999"})
	assert.False(t, found)
}

func TestPendingRegistry_Remove(t *testing.T) {
	reg := NewPendingRegistry(5 * time.Second)
	reg.Register(context.Background(), "789")
	assert.Equal(t, 1, reg.Len())
	reg.Remove("789")
	assert.Equal(t, 0, reg.Len())
}
