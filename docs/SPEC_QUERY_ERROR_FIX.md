# 规格查询错误不可重试修复

## 问题描述

当AI生成的规格名称在TEMU模板中不存在时，规格查询失败（error_code=10000019），但错误被标记为可重试，导致任务反复重试：

```
ERRO 规格查询失败 [Color/红色]: 规格查询失败: error_code=10000019
INFO 错误类型: *fmt.wrapError, 是否可重试: true
```

**问题：** 这是数据问题（规格不存在），不是临时性错误，重试也不会成功。

## 根本原因

在 `platforms/temu/handlers/sku_spec_query.go` 第76行：

```go
if !response.Success {
    return "", fmt.Errorf("规格查询失败: error_code=%d", response.ErrorCode)
}
```

**问题：**
- 所有错误都返回相同的错误格式
- 没有区分数据错误和临时性错误
- 错误码10000019（规格不存在）应该是不可重试的

## 解决方案

### 修改 `platforms/temu/handlers/sku_spec_query.go`

**修改前：**
```go
if !response.Success {
    return "", fmt.Errorf("规格查询失败: error_code=%d", response.ErrorCode)
}
```

**修改后：**
```go
if !response.Success {
    // 错误码10000019表示规格不存在或无效，这是数据问题，不应该重试
    if response.ErrorCode == 10000019 {
        sb.logger.Errorf("❌ 规格查询失败: 规格'%s'在TEMU模板中不存在 (parent_spec_id=%s, error_code=%d)", 
            specName, parentSpecID, response.ErrorCode)
        sb.logger.Error("💡 可能的原因:")
        sb.logger.Error("   1. AI生成的规格名称与TEMU模板不匹配")
        sb.logger.Error("   2. parent_spec_id不正确")
        sb.logger.Error("   3. 需要在TEMU模板中添加这个规格值")
        return "", fmt.Errorf("NONRETRYABLE: 规格'%s'不存在于TEMU模板中 (error_code=%d)", specName, response.ErrorCode)
    }
    return "", fmt.Errorf("规格查询失败: error_code=%d", response.ErrorCode)
}
```

## 效果对比

### 修改前
```
ERRO 规格查询失败 [Color/红色]: 规格查询失败: error_code=10000019
INFO 错误类型: *fmt.wrapError, 是否可重试: true
WARN 任务重新入队，等待重试
... (反复重试)
```

### 修改后
```
ERRO ❌ 规格查询失败: 规格'红色'在TEMU模板中不存在 (parent_spec_id=1001, error_code=10000019)
ERRO 💡 可能的原因:
ERRO    1. AI生成的规格名称与TEMU模板不匹配
ERRO    2. parent_spec_id不正确
ERRO    3. 需要在TEMU模板中添加这个规格值
ERRO 规格查询失败 [Color/红色]: NONRETRYABLE: 规格'红色'不存在于TEMU模板中 (error_code=10000019)
INFO 错误类型: *fmt.wrapError, 是否可重试: false
INFO 任务标记为失败，不再重试 ✅
```

## 常见错误码

| 错误码 | 含义 | 是否可重试 |
|--------|------|-----------|
| 10000019 | 规格不存在或无效 | ❌ 不可重试 |
| 10000103 | SKU重复 | ❌ 不可重试 |
| 503 | 服务不可用 | ✅ 可重试 |
| 超时 | 网络超时 | ✅ 可重试 |

## 如何解决规格不存在问题

### 方案1：修改AI Prompt
确保AI从TEMU模板的可选值中选择规格，而不是自己创建。

### 方案2：添加规格映射
在代码中添加规格名称映射：
```go
// 中文到英文的映射
specNameMap := map[string]string{
    "红色": "Red",
    "蓝色": "Blue",
    "黑色": "Black",
}
```

### 方案3：在TEMU模板中添加规格
如果规格确实需要，在TEMU后台添加对应的规格值。

## 相关文件

- `platforms/temu/handlers/sku_spec_query.go` - 规格查询逻辑
- `platforms/temu/errors.go` - 错误类型定义
- `platforms/temu/handlers/sku_ai_mapping.go` - AI规格生成
