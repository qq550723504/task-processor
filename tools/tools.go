//go:build tools
// +build tools

// Package tools 管理项目开发和构建所需的工具依赖
// 这个文件不会被编译到最终的二进制文件中
package tools

import (
	// 代码检查工具
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"

	// Mock 生成工具
	_ "github.com/golang/mock/mockgen"

	// Swagger 文档生成
	_ "github.com/swaggo/swag/cmd/swag"

	// 数据库迁移工具
	_ "github.com/golang-migrate/migrate/v4/cmd/migrate"
)

// 安装所有工具:
// go install $(go list -f '{{join .Imports " "}}' tools/tools.go)
