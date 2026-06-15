package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/tenantctx"
	sheinpub "task-processor/internal/publishing/shein"
)

func (r *taskRepository) CreateTask(ctx context.Context, task *listingkit.Task) error {
	if task != nil {
		if task.TenantID == "" {
			task.TenantID = tenantctx.TenantIDFromContext(ctx)
		}
		if task.UserID == "" {
			task.UserID = listingkit.ResolveTaskUserID(task)
		}
		if task.Request != nil && task.Request.TenantID == "" {
			task.Request.TenantID = task.TenantID
		}
		if task.Request != nil && task.Request.UserID == "" {
			task.Request.UserID = task.UserID
		}
	}
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *taskRepository) GetTask(ctx context.Context, taskID string) (*listingkit.Task, error) {
	var task listingkit.Task
	db := applyTaskAccessScope(r.db.WithContext(ctx), ctx)
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, listingkit.ErrTaskNotFound
		}
		return nil, err
	}
	if !taskVisibleToUser(ctx, &task) {
		return nil, listingkit.ErrTaskNotFound
	}
	return &task, nil
}

func (r *taskRepository) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, int64, error) {
	page, pageSize := normalizeTaskListPage(query)
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	if query != nil && query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	if query != nil && (query.Platform != "" || query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "") {
		var candidates []taskListFilterRow
		columns := []string{"id", "created_at", "status", "user_id"}
		if query.Platform != "" || query.SheinWorkQueue != "" {
			columns = append(columns, "request")
		}
		if query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "" {
			columns = append(columns, "result")
		}
		if err := db.Select(columns).Order("created_at DESC").Find(&candidates).Error; err != nil {
			return nil, 0, err
		}
		filteredIDs := make([]string, 0, len(candidates))
		for i := range candidates {
			if !taskVisibleToUser(ctx, &listingkit.Task{UserID: candidates[i].UserID, Request: &listingkit.GenerateRequest{UserID: candidates[i].RequestUserID}}) {
				continue
			}
			if !matchesTaskListFilterRow(&candidates[i], query) {
				continue
			}
			filteredIDs = append(filteredIDs, candidates[i].ID)
		}
		total := int64(len(filteredIDs))
		start := (page - 1) * pageSize
		if start >= len(filteredIDs) {
			return []listingkit.Task{}, total, nil
		}
		end := start + pageSize
		if end > len(filteredIDs) {
			end = len(filteredIDs)
		}
		return r.loadTasksByIDs(ctx, filteredIDs[start:end], total)
	}

	var total int64
	var tasks []listingkit.Task
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (r *taskRepository) ListTaskSummaryTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, error) {
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	if query != nil && query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	if query != nil && (query.Platform != "" || query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "") {
		var candidates []taskListFilterRow
		columns := []string{"id", "created_at", "status", "user_id"}
		if query.Platform != "" || query.SheinWorkQueue != "" {
			columns = append(columns, "request")
		}
		if query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "" {
			columns = append(columns, "result")
		}
		if err := db.Select(columns).Order("created_at DESC").Find(&candidates).Error; err != nil {
			return nil, err
		}
		filteredIDs := make([]string, 0, len(candidates))
		for i := range candidates {
			if !taskVisibleToUser(ctx, &listingkit.Task{UserID: candidates[i].UserID, Request: &listingkit.GenerateRequest{UserID: candidates[i].RequestUserID}}) {
				continue
			}
			if !matchesTaskListFilterRow(&candidates[i], query) {
				continue
			}
			filteredIDs = append(filteredIDs, candidates[i].ID)
		}
		tasks, _, err := r.loadTasksByIDs(ctx, filteredIDs, int64(len(filteredIDs)))
		return tasks, err
	}

	var tasks []listingkit.Task
	if err := db.Order("created_at DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return filterTasksForUser(ctx, tasks), nil
}

type taskListFilterRow struct {
	ID            string    `gorm:"column:id"`
	UserID        string    `gorm:"column:user_id"`
	Status        string    `gorm:"column:status"`
	Request       string    `gorm:"column:request"`
	Result        string    `gorm:"column:result"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	RequestUserID string    `gorm:"-"`
}

type taskListFilterRequest struct {
	Platforms []string `json:"platforms,omitempty"`
	UserID    string   `json:"user_id,omitempty"`
}

type taskListFilterResult struct {
	Shein        *sheinpub.Package               `json:"shein,omitempty"`
	PodExecution *listingkit.PodExecutionSummary `json:"pod_execution,omitempty"`
}

func matchesTaskListFilterRow(row *taskListFilterRow, query *listingkit.TaskListQuery) bool {
	if row == nil {
		return false
	}
	task := &listingkit.Task{}
	task.Status = listingkit.TaskStatus(row.Status)
	if query != nil && (query.Platform != "" || query.SheinWorkQueue != "") {
		var request taskListFilterRequest
		if err := json.Unmarshal([]byte(row.Request), &request); err == nil {
			task.Request = &listingkit.GenerateRequest{Platforms: request.Platforms, UserID: request.UserID}
			row.RequestUserID = request.UserID
		}
	}
	if task.Request == nil {
		task.Request = &listingkit.GenerateRequest{UserID: row.RequestUserID}
	}
	if query != nil && (query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "") {
		var result taskListFilterResult
		if err := json.Unmarshal([]byte(row.Result), &result); err == nil {
			task.Result = &listingkit.ListingKitResult{Shein: result.Shein, PodExecution: result.PodExecution}
		}
	}
	return listingkit.TaskMatchesListQuery(task, query)
}

func (r *taskRepository) loadTasksByIDs(ctx context.Context, ids []string, total int64) ([]listingkit.Task, int64, error) {
	if len(ids) == 0 {
		return []listingkit.Task{}, total, nil
	}
	var tasks []listingkit.Task
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	if err := db.Where("id IN ?", ids).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	tasks = filterTasksForUser(ctx, tasks)
	order := make(map[string]int, len(ids))
	for i, id := range ids {
		order[id] = i
	}
	ordered := make([]listingkit.Task, 0, len(tasks))
	for _, task := range tasks {
		index, ok := order[task.ID]
		if !ok {
			continue
		}
		if len(ordered) <= index {
			next := make([]listingkit.Task, index+1)
			copy(next, ordered)
			ordered = next
		}
		ordered[index] = task
	}
	compacted := make([]listingkit.Task, 0, len(tasks))
	for _, task := range ordered {
		if task.ID == "" {
			continue
		}
		compacted = append(compacted, task)
	}
	return compacted, total, nil
}
