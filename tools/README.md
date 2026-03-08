# 工具依赖目录

## 用途

管理项目开发和构建所需的 Go 工具依赖。

## tools.go 文件

```go
// +build tools

package tools

import (
    _ "github.com/golangci/golangci-lint/cmd/golangci-lint"
    _ "github.com/swaggo/swag/cmd/swag"
    _ "github.com/golang/mock/mockgen"
    _ "google.golang.org/protobuf/cmd/protoc-gen-go"
    _ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)
```

## 为什么需要 tools.go？

1. **版本锁定**：通过 go.mod 锁定工具版本
2. **团队一致**：确保团队使用相同版本的工具
3. **CI/CD**：在 CI 环境中安装正确版本的工具

## 安装工具

```bash
# 安装所有工具
go install $(go list -f '{{join .Imports " "}}' tools/tools.go)

# 或者单独安装
go install github.com/golangci/golangci-lint/cmd/golangci-lint
go install github.com/swaggo/swag/cmd/swag
```

## 常用工具

- **golangci-lint**：代码检查
- **swag**：生成 Swagger 文档
- **mockgen**：生成 mock 代码
- **protoc-gen-go**：生成 protobuf 代码
- **wire**：依赖注入代码生成

## 最佳实践

1. 将开发工具作为项目依赖管理
2. 在 Makefile 中使用这些工具
3. 在 CI/CD 中安装这些工具
4. 定期更新工具版本
