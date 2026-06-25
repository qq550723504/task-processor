package managementapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskStatusSnapshotFromDTOCopiesTaskStatusFields(t *testing.T) {
	t.Parallel()

	snapshot := TaskStatusSnapshotFromDTO(&TaskStatusRespDTO{
		TaskID:           123,
		Status:           "2",
		StatusKey:        "processing",
		StatusName:       "Processing",
		CanonicalStatus:  "running",
		Platform:         "shein",
		Region:           "US",
		TaskType:         "publish",
		Priority:         7,
		RetryCount:       1,
		MaxRetries:       3,
		ProcessingTimeMs: 456,
		QueueName:        "queue-a",
		ProcessingNode:   "node-1",
		ProgressPercent:  42,
		Result:           `{"ok":true}`,
		ErrorMessage:     "boom",
		ErrorStack:       "stack",
		ExecutionLogs:    []string{"log-a", "log-b"},
		TaskDetails:      `{"sku":"SKU1"}`,
	})

	require.NotNil(t, snapshot)
	require.Equal(t, int64(123), snapshot.TaskID)
	require.Equal(t, "2", snapshot.Status)
	require.Equal(t, "processing", snapshot.StatusKey)
	require.Equal(t, "Processing", snapshot.StatusName)
	require.Equal(t, "running", snapshot.CanonicalStatus)
	require.Equal(t, "shein", snapshot.Platform)
	require.Equal(t, "US", snapshot.Region)
	require.Equal(t, "publish", snapshot.TaskType)
	require.Equal(t, 7, snapshot.Priority)
	require.Equal(t, 1, snapshot.RetryCount)
	require.Equal(t, 3, snapshot.MaxRetries)
	require.Equal(t, int64(456), snapshot.ProcessingTimeMs)
	require.Equal(t, "queue-a", snapshot.QueueName)
	require.Equal(t, "node-1", snapshot.ProcessingNode)
	require.Equal(t, 42, snapshot.ProgressPercent)
	require.Equal(t, `{"ok":true}`, snapshot.Result)
	require.Equal(t, "boom", snapshot.ErrorMessage)
	require.Equal(t, "stack", snapshot.ErrorStack)
	require.Equal(t, []string{"log-a", "log-b"}, snapshot.ExecutionLogs)
	require.Equal(t, `{"sku":"SKU1"}`, snapshot.TaskDetails)
	require.Nil(t, TaskStatusSnapshotFromDTO(nil))
}
