# 错误处理最佳实践指南

## 概述

本项目已经有一个完善的错误处理包 `internal/core/errors`,本指南说明如何正确使用它。

## 核心功能

### 1. 创建错误

```go
import "task-processor/internal/core/errors"

// 简单错误
err := errors.New(errors.ErrCodeValidation, "invalid input")

// 格式化错误
err := errors.Newf(errors.ErrCodeNotFound, "task %s not found", taskID)
```

### 2. 包装错误(保留错误链)

```go
// 包装现有错误,添加上下文
task, err := repo.GetTask(ctx, taskID)
if err != nil {
    return errors.Wrap(err, errors.ErrCodeNotFound, 
        "failed to get task").
        WithDetails(fmt.Sprintf("task_id=%s", taskID))
}
```

### 3. 添加详细信息

```go
err := errors.Wrap(err, errors.ErrCodeInternalError, "processing failed").
    WithDetails(fmt.Sprintf("platform=%s, retry_count=%d", platform, retryCount)).
    WithStack()  // 可选:添加堆栈跟踪
```

### 4. 检查错误类型

```go
var appErr *errors.AppError
if errors.As(err, &appErr) {
    switch appErr.Code {
    case errors.ErrCodeNotFound:
        // 处理未找到
    case errors.ErrCodeValidation:
        // 处理验证错误
    }
}

// 或者使用 errors.Is
if errors.Is(err, errors.ErrCodeTimeout) {
    // 处理超时
}
```

## 实际示例

### Task Submission 服务

```go
// internal/listingkit/task_submission_service.go

func (s *taskSubmissionService) SubmitTask(ctx context.Context, taskID string, req *SubmitRequest) error {
    // 获取任务
    task, err := s.repo.GetTask(ctx, taskID)
    if err != nil {
        return errors.Wrap(err, errors.ErrCodeTaskNotFound, 
            "failed to get task for submission").
            WithDetails(fmt.Sprintf("task_id=%s", taskID))
    }
    
    // 验证任务状态
    if task.Status != TaskStatusReadyForReview {
        return errors.New(errors.ErrCodeTaskProcessing,
            "task is not ready for submission").
            WithDetails(fmt.Sprintf("current_status=%s", task.Status))
    }
    
    // 构建提交请求
    buildReq, err := s.buildSubmitRequest(task, req)
    if err != nil {
        return errors.Wrap(err, errors.ErrCodeValidation,
            "failed to build submit request").
            WithDetails(fmt.Sprintf("task_id=%s, platform=%s", taskID, req.Platform))
    }
    
    // 提交到平台
    result, err := s.platformClient.Submit(ctx, buildReq)
    if err != nil {
        return errors.Wrap(err, errors.ErrCodePlatformError,
            "platform submission failed").
            WithDetails(fmt.Sprintf("task_id=%s, platform=%s", taskID, req.Platform)).
            WithStack()  // 关键路径添加堆栈
    }
    
    // 更新任务状态
    if err := s.updateTaskStatus(ctx, taskID, result); err != nil {
        return errors.Wrap(err, errors.ErrCodeSystem,
            "failed to update task status after submission").
            WithDetails(fmt.Sprintf("task_id=%s", taskID))
    }
    
    return nil
}
```

### Consumer 层

```go
// internal/app/consumer/task_handler.go

func (h *TaskHandler) HandleTask(task *model.Task) error {
    err := h.processor.Process(task)
    if err != nil {
        appErr := errors.Wrap(err, errors.ErrCodeSystem,
            "task processing failed").
            WithDetails(fmt.Sprintf("task_id=%s, platform=%s, retry_count=%d",
                task.ID, task.Platform, task.RetryCount))
        
        h.logger.WithFields(logrus.Fields{
            "task_id":     task.ID,
            "platform":    task.Platform,
            "error_code":  appErr.Code,
            "retry_count": task.RetryCount,
            "details":     appErr.Details,
        }).Errorf("task failed: %v", appErr)
        
        return appErr
    }
    return nil
}
```

### HTTP API 层

```go
// internal/listingkit/httpapi/error_handler.go

func HandleError(c *gin.Context, err error) {
    var appErr *errors.AppError
    if errors.As(err, &appErr) {
        statusCode := httpStatusCodeFromErrorCode(appErr.Code)
        
        c.JSON(statusCode, gin.H{
            "code":    appErr.Code,
            "message": appErr.Message,
            "details": appErr.Details,
            "timestamp": appErr.Timestamp,
        })
        return
    }
    
    // 未知错误
    c.JSON(http.StatusInternalServerError, gin.H{
        "code":    errors.ErrCodeSystem,
        "message": "internal server error",
        "timestamp": time.Now(),
    })
}

func httpStatusCodeFromErrorCode(code errors.ErrorCode) int {
    switch code {
    case errors.ErrCodeTaskNotFound:
        return http.StatusNotFound
    case errors.ErrCodeValidation:
        return http.StatusBadRequest
    case errors.ErrCodeAuth:
        return http.StatusUnauthorized
    default:
        return http.StatusInternalServerError
    }
}
```

## 错误代码列表

### 系统级错误
- `ErrCodeSystem` - 系统内部错误
- `ErrCodeConfig` - 配置错误
- `ErrCodeAuth` - 认证/授权错误
- `ErrCodeNetwork` - 网络错误
- `ErrCodeTimeout` - 超时错误
- `ErrCodeResourceLimit` - 资源限制错误

### 业务级错误
- `ErrCodeTaskNotFound` - 任务未找到
- `ErrCodeTaskDuplicate` - 任务重复
- `ErrCodeTaskProcessing` - 任务处理中
- `ErrCodePlatformError` - 平台错误
- `ErrCodeValidation` - 验证错误

### 外部服务错误
- `ErrCodeExternalAPI` - 外部 API 错误
- `ErrCodeAmazonAPI` - Amazon API 错误
- `ErrCodeManagementAPI` - 管理 API 错误
- `ErrCodeOpenAIAPI` - OpenAI API 错误

## 最佳实践

1. **始终包装错误**: 在边界处(如 repository、external API)包装错误,添加上下文
2. **使用 WithDetails**: 添加关键信息如 taskID、platform、userID 等
3. **关键路径加堆栈**: 对于难以调试的错误,使用 `.WithStack()`
4. **记录日志**: 在 consumer 层记录完整的错误信息
5. **统一响应**: 在 HTTP API 层统一转换错误为 JSON 响应
6. **不要吞掉错误**: 始终返回或记录错误,不要忽略

## 迁移指南

如果要改进现有代码的错误处理:

1. 识别缺少上下文的错误返回点
2. 使用 `errors.Wrap` 包装,添加 Details
3. 在日志中添加 error_code 和 details 字段
4. 确保 HTTP handler 使用统一的错误响应

## 参考

- 完整 API 文档: `internal/core/errors/errors.go`
- 使用示例: `internal/core/errors/examples_test.go`
- 测试用例: `internal/core/errors/errors_test.go`
