# 公共库目录

## 用途

存放可以被外部项目导入的公共库代码。

## 与 internal/pkg 的区别

- **pkg/**：可以被外部项目导入的公共库
- **internal/pkg/**：只能在项目内部使用的公共包

## 目录结构

```
pkg/
├── middleware/          # 公共中间件
│   ├── auth.go
│   ├── logging.go
│   └── recovery.go
├── response/            # 统一响应格式
│   └── response.go
├── pagination/          # 分页工具
│   └── pagination.go
└── validator/           # 验证器
    └── validator.go
```

## 使用场景

适合放在 pkg/ 的代码：
- 通用的中间件
- 统一的响应格式
- 分页工具
- 通用的验证器
- 工具函数库

## 示例

```go
// pkg/response/response.go
package response

type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
    return Response{
        Code:    0,
        Message: "success",
        Data:    data,
    }
}

func Error(code int, message string) Response {
    return Response{
        Code:    code,
        Message: message,
    }
}
```

## 最佳实践

1. 只放置高度通用的代码
2. 不依赖 internal 包
3. 保持 API 稳定
4. 提供完整的文档
5. 考虑向后兼容性
