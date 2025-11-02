# TEMU API客户端 Cookie管理

## 功能概述

TEMU API客户端现在支持自动cookie获取和加载功能，包括：

- 自动从管理系统获取cookie
- 解析cookie字符串为http.Cookie对象
- 在API请求中自动添加cookie
- 支持cookie重新加载

## 使用方法

### 1. 创建API客户端（自动加载cookie）

```go
// 创建API客户端，会自动从管理系统加载cookie
apiClient := temu.NewAPIClient(tenantID, storeID, managementClient)

// 检查是否有cookie
if apiClient.HasCookies() {
    fmt.Printf("已加载 %d 个cookie\n", apiClient.GetCookieCount())
}
```

### 2. 手动重新加载cookie

```go
// 重新从管理系统加载cookie
err := apiClient.ReloadCookies()
if err != nil {
    log.Printf("重新加载cookie失败: %v", err)
}
```

### 3. 手动设置cookie

```go
cookies := []*http.Cookie{
    {Name: "session_id", Value: "abc123"},
    {Name: "user_token", Value: "xyz789"},
}
apiClient.SetCookies(cookies)
```

## Cookie格式

管理系统返回的cookie字符串格式应为：
```
session_id=abc123; user_token=xyz789; lang=zh-CN
```

系统会自动解析并设置以下默认属性：
- Domain: `.temu.com`
- Path: `/`

## 错误处理

- 如果管理系统中没有cookie，客户端会正常工作但不会添加cookie到请求中
- 无效的cookie格式会被跳过，并记录警告日志
- Cookie加载失败会记录错误日志，但不会阻止API客户端的创建
- 认证失败（用户token为空或过期）会导致cookie加载失败

## 调试Cookie问题

如果TextCheckHandler执行时cookie为空，可以通过以下方式排查：

### 1. 检查日志
查看以下关键日志信息：
- `管理系统连接测试失败` - 表示用户token问题
- `从管理系统获取Cookie失败` - 表示API调用失败
- `未找到Cookie数据` - 表示数据库中没有cookie
- `API客户端状态` - 显示cookie加载状态

### 2. 使用调试工具
```go
// 在代码中添加调试
temu.DebugCookieIssue(storeID, managementClient)
```

### 3. 常见问题及解决方案

**问题1：访问令牌为空**
- 确保用户已登录并设置了token
- 检查`SetUserToken`是否被正确调用

**问题2：Cookie数据为空**
- 检查数据库中是否存在对应storeID的cookie
- 确认cookie没有被删除或过期

**问题3：解析Cookie失败**
- 检查cookie字符串格式是否正确
- 确认cookie字符串不包含特殊字符

## 测试

运行测试：
```bash
go test ./common/temu/ -v
```