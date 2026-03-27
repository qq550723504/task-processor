package productenrich

import "context"

type mockTaskRepo struct {
	tasks        map[string]*Task
	retryErr     error
	resetErr     error
	incrementErr error
}

func newMockTaskRepo(tasks ...*Task) *mockTaskRepo {
	r := &mockTaskRepo{tasks: make(map[string]*Task)}
	for _, t := range tasks {
		r.tasks[t.ID] = t
	}
	return r
}

func (r *mockTaskRepo) CreateTask(_ context.Context, task *Task) error {
	r.tasks[task.ID] = task
	return nil
}

func (r *mockTaskRepo) GetTask(_ context.Context, id string) (*Task, error) {
	t, ok := r.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

func (r *mockTaskRepo) MarkProcessing(_ context.Context, id string) error {
	if t, ok := r.tasks[id]; ok {
		t.Status = TaskStatusProcessing
		t.Error = ""
	}
	return nil
}

func (r *mockTaskRepo) UpdateTaskStatus(_ context.Context, id string, status TaskStatus) error {
	if t, ok := r.tasks[id]; ok {
		t.Status = status
	}
	return nil
}

func (r *mockTaskRepo) MarkFailed(_ context.Context, id string, msg string) error {
	if t, ok := r.tasks[id]; ok {
		t.Error = msg
		t.Status = TaskStatusFailed
	}
	return nil
}

func (r *mockTaskRepo) UpdateTaskError(_ context.Context, id string, msg string) error {
	if t, ok := r.tasks[id]; ok {
		t.Error = msg
		t.Status = TaskStatusFailed
	}
	return nil
}

func (r *mockTaskRepo) MarkCompleted(_ context.Context, id string, result *ProductJSON) error {
	if t, ok := r.tasks[id]; ok {
		t.Result = result
		t.Status = TaskStatusCompleted
		t.Error = ""
	}
	return nil
}

func (r *mockTaskRepo) SaveTaskResult(_ context.Context, id string, result *ProductJSON) error {
	if t, ok := r.tasks[id]; ok {
		t.Result = result
		t.Status = TaskStatusCompleted
	}
	return nil
}

func (r *mockTaskRepo) IncrementRetryCount(_ context.Context, id string) error {
	if r.incrementErr != nil {
		return r.incrementErr
	}
	if t, ok := r.tasks[id]; ok {
		t.RetryCount++
	}
	return nil
}

func (r *mockTaskRepo) PrepareRetry(_ context.Context, id string) error {
	if r.resetErr != nil {
		return r.resetErr
	}
	if t, ok := r.tasks[id]; ok {
		t.Status = TaskStatusPending
		t.Error = ""
	}
	return nil
}

func (r *mockTaskRepo) ResetForRetry(_ context.Context, id string) error {
	if r.resetErr != nil {
		return r.resetErr
	}
	if t, ok := r.tasks[id]; ok {
		t.Status = TaskStatusPending
		t.Error = ""
	}
	return nil
}
