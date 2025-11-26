# Cookie为空时暂停任务修复

## 问题描述

当店铺的Cookie数据为空时，系统会：
1. 尝试重新加载Cookie
2. 设置认证过期暂停键
3. 但任务仍被标记为`pending_retry`状态
4. 任务会继续重试，导致重复的错误日志

**期望行为**：当Cookie为空时，应该暂停该店铺的所有任务，等待管理员在管理系统中设置Cookie后再恢复。

## 错误日志示例

```
WARN[2025-11-22 17:54:53] 店铺ID=508没有Cookie数据，尝试重新加载Cookie
ERRO[2025-11-22 17:54:53] 重新加载Cookie失败: 从管理系统获取Cookie失败: Cookie数据为空
INFO[2025-11-22 17:54:53] 设置店铺 508 的认证过期暂停键，原因: 从管理系统获取Cookie失败: Cookie数据为空
INFO[2025-11-22 17:54:53] ✓ 成功设置店铺 508 的认证过期暂停键
ERRO[2025-11-22 17:54:53] 发送请求失败: 店铺ID=508没有Cookie数据且重新加载失败
INFO[2025-11-22 17:54:53] 错误类型: *fmt.wrapError, 是否可重试: true
INFO[2025-11-22 17:54:53] 准备同步更新任务状态: TaskID=2051971, Status=pending_retry
WARN[2025-11-22 17:54:53] ⚠️ 任务处理失败，等待重试: ID=2051971
```

## 解决方案

### 1. 创建AuthExpiredError错误类型

在`common/temu/errors.go`中定义了新的错误类型`AuthExpiredError`，用于标识认证过期（Cookie为空）的情况。

```go
type AuthExpiredError struct {
    Message string
    Cause   error
}
```

### 2. 修改API客户端返回AuthExpiredError

在`common/temu/api_client.go`的`SendTEMURequest`方法中，当检测到Cookie为空时，返回`AuthExpiredError`而不是普通错误：

```go
// 返回AuthExpiredError以便任务处理器识别并暂停任务
return NewAuthExpiredError(
    fmt.Sprintf("店铺ID=%d没有Cookie数据，请先在管理系统中设置Cookie", c.GetStoreID()),
    nil,
)
```

### 3. 添加IsAuthExpiredError判断函数

在`platforms/temu/errors.go`中添加了`IsAuthExpiredError`函数，用于识别认证过期错误：

```go
func IsAuthExpiredError(err error) bool {
    // 检查是否为AuthExpiredError类型
    if _, ok := err.(*temuCommon.AuthExpiredError); ok {
        return true
    }
    
    // 检查错误内容是否包含Cookie相关关键词
    cookieErrors := []string{
        "Cookie数据为空",
        "没有Cookie数据",
        "从管理系统获取Cookie失败",
        "请先在管理系统中设置Cookie",
    }
    // ...
}
```

### 4. 修改任务处理器的错误处理逻辑

在`platforms/temu/task_handler.go`的`handleTaskFailure`方法中，优先检查是否为认证过期错误，如果是则将任务状态设置为`paused`：

```go
func (h *TaskHandler) handleTaskFailure(task types.Task, err error) {
    // 首先检查是否为认证过期错误（Cookie为空）
    isAuthExpired := IsAuthExpiredError(err)
    if isAuthExpired {
        h.logger.Infof("错误类型: %T, 错误值: %v, 是否为认证过期: true", err, err)
        // 认证过期错误，暂停任务等待Cookie更新
        h.updateTaskStatusSync(task.ID, "paused", err.Error())
        h.logger.Warnf("⏸️ 任务因认证过期而暂停: ID=%s, StoreID=%d, 错误=%v", task.ID, task.StoreID, err)
        return
    }
    // ... 其他错误处理逻辑
}
```

### 5. 添加paused状态映射

在`updateTaskStatusSync`方法中添加了对`paused`状态的映射：

```go
case "paused":
    statusCode = types.TaskStatusPaused.Int16() // 已暂停
```

## 修改的文件

1. `common/temu/errors.go` - 新建文件，定义AuthExpiredError
2. `common/temu/api_client.go` - 修改Cookie检查逻辑，返回AuthExpiredError
3. `platforms/temu/errors.go` - 添加IsAuthExpiredError函数
4. `platforms/temu/task_handler.go` - 修改错误处理逻辑，支持暂停任务

## 预期效果

修复后的日志应该显示：

```
WARN[2025-11-22 17:54:53] 店铺ID=508没有Cookie数据，尝试重新加载Cookie
ERRO[2025-11-22 17:54:53] 重新加载Cookie失败: 从管理系统获取Cookie失败: Cookie数据为空
INFO[2025-11-22 17:54:53] 设置店铺 508 的认证过期暂停键，原因: 从管理系统获取Cookie失败: Cookie数据为空
INFO[2025-11-22 17:54:53] ✓ 成功设置店铺 508 的认证过期暂停键
INFO[2025-11-22 17:54:53] 错误类型: *temu.AuthExpiredError, 是否为认证过期: true
INFO[2025-11-22 17:54:53] 准备同步更新任务状态: TaskID=2051971, Status=paused
WARN[2025-11-22 17:54:53] ⏸️ 任务因认证过期而暂停: ID=2051971, StoreID=508
INFO[2025-11-22 17:54:53] ✅ 任务状态同步更新成功: TaskID=2051971, Status=paused
```

## 后续处理

当管理员在管理系统中为店铺508设置了Cookie后：
1. 系统会清除认证过期暂停键
2. 暂停的任务可以被恢复处理
3. 任务会重新获取Cookie并继续执行
