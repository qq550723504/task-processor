package management

import (
	"fmt"

	"task-processor/internal/model"
)

type localTaskStatusMeta struct {
	Key       string
	Name      string
	Canonical string
}

func localTaskStatusMetadata(status int16) localTaskStatusMeta {
	switch model.TaskStatus(status) {
	case model.TaskStatusPending:
		return localTaskStatusMeta{Key: "PENDING", Name: "待处理", Canonical: "pending"}
	case model.TaskStatusProcessing:
		return localTaskStatusMeta{Key: "PROCESSING", Name: "处理中", Canonical: "processing"}
	case model.TaskStatusCrawled:
		return localTaskStatusMeta{Key: "CRAWLED", Name: "已抓取", Canonical: "completed"}
	case model.TaskStatusCrawlFailed:
		return localTaskStatusMeta{Key: "CRAWL_FAILED", Name: "抓取失败", Canonical: "failed"}
	case model.TaskStatusPendingRetry:
		return localTaskStatusMeta{Key: "PENDING_RETRY", Name: "待重试", Canonical: "pending"}
	case model.TaskStatusQueued:
		return localTaskStatusMeta{Key: "QUEUED", Name: "队列中", Canonical: "pending"}
	case model.TaskStatusPublished:
		return localTaskStatusMeta{Key: "PUBLISHED", Name: "已上架", Canonical: "completed"}
	case model.TaskStatusRepublishing:
		return localTaskStatusMeta{Key: "REPUBLISHING", Name: "重新上架中", Canonical: "processing"}
	case model.TaskStatusDraft:
		return localTaskStatusMeta{Key: "DRAFT", Name: "草稿箱", Canonical: "completed"}
	case model.TaskStatusCancelled:
		return localTaskStatusMeta{Key: "CANCELLED", Name: "已取消", Canonical: "cancelled"}
	case model.TaskStatusPaused:
		return localTaskStatusMeta{Key: "PAUSED", Name: "已暂停", Canonical: "paused"}
	case model.TaskStatusResumed:
		return localTaskStatusMeta{Key: "RESUMED", Name: "已恢复", Canonical: "processing"}
	case model.TaskStatusResuming:
		return localTaskStatusMeta{Key: "RESUMING", Name: "恢复中", Canonical: "processing"}
	case model.TaskStatusTerminated:
		return localTaskStatusMeta{Key: "TERMINATED", Name: "已终止", Canonical: "failed"}
	default:
		return localTaskStatusMeta{Key: fmt.Sprintf("STATUS_%d", status), Name: fmt.Sprintf("状态%d", status), Canonical: "unknown"}
	}
}
