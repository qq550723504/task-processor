# Go 最佳实践修复工具和脚本

## 🛠️ 自动化修复脚本

### 1. 错误处理修复脚本

**文件**: `scripts/fix_error_handling.sh`

```bash
#!/bin/bash
# 修复错误处理中的%v为%w

echo "修复错误处理中的%v为%w..."

# 使用sed进行全局替换
find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" | while read file; do
    # 替换fmt.Errorf中的%v为%w
    sed -i 's/fmt\.Errorf("\([^"]*\): %v"/fmt.Errorf("\1: %w"/g' "$file"
    
    # 替换其他错误包装中的%v为%w
    sed -i 's/errors\.New("\([^"]*\): %v"/errors.New("\1: %w"/g' "$file"
done

echo "✅ 错误处理修复完成"
```

**使用方法**:
```bash
chmod +x scripts/fix_error_handling.sh
./scripts/fix_error_handling.sh
```

---

### 2. Goroutine Panic Recovery修复脚本

**文件**: `scripts/fix_goroutine_panic.sh`

```bash
#!/bin/bash
# 检查并报告缺少panic recovery的goroutine

echo "检查缺少panic recovery的goroutine..."

grep -r "go func()" --include="*.go" . | grep -v "defer.*recover" | while read line; do
    file=$(echo "$line" | cut -d: -f1)
    linenum=$(echo "$line" | cut -d: -f2)
    echo "⚠️  $file:$linenum - 缺少panic recovery"
done

echo ""
echo "修复方案: 为每个goroutine添加以下代码"
echo ""
echo "go func() {"
echo "    defer func() {"
echo "        if r := recover(); r != nil {"
echo "            logrus.Errorf(\"goroutine panic recovered: %v\", r)"
echo "        }"
echo "    }()"
echo "    // 业务逻辑"
echo "}()"
```

**使用方法**:
```bash
chmod +x scripts/fix_goroutine_panic.sh
./scripts/fix_goroutine_panic.sh
```

---

### 3. Context使用检查脚本

**文件**: `scripts/check_context_usage.sh`

```bash
#!/bin/bash
# 检查context.Background()的不当使用

echo "检查context.Background()的不当使用..."

grep -r "context\.Background()" --include="*.go" . | grep -v "context.WithCancel\|context.WithTimeout\|context.WithDeadline" | while read line; do
    file=$(echo "$line" | cut -d: -f1)
    linenum=$(echo "$line" | cut -d: -f2)
    echo "⚠️  $file:$linenum - 不当使用context.Background()"
done

echo ""
echo "修复方案: 通过参数接收context，而不是使用context.Background()"
```

**使用方法**:
```bash
chmod +x scripts/check_context_usage.sh
./scripts/check_context_usage.sh
```

---

### 4. 文件长度检查脚本

**文件**: `scripts/check_file_length.sh`

```bash
#!/bin/bash
# 检查超过300行的文件

echo "检查超过300行的文件..."

find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" | while read file; do
    lines=$(wc -l < "$file")
    if [ $lines -gt 300 ]; then
        echo "🔴 $file: $lines行"
    elif [ $lines -gt 200 ]; then
        echo "🟡 $file: $lines行"
    fi
done
```

**使用方法**:
```bash
chmod +x scripts/check_file_length.sh
./scripts/check_file_length.sh
```

---

### 5. 包注释检查脚本

**文件**: `scripts/check_package_comments.sh`

```bash
#!/bin/bash
# 检查缺少包注释的文件

echo "检查缺少包注释的文件..."

find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" ! -name "*_test.go" | while read file; do
    # 检查文件前5行是否包含包注释
    if ! head -5 "$file" | grep -q "^// Package"; then
        echo "⚠️  $file - 缺少包注释"
    fi
done
```

**使用方法**:
```bash
chmod +x scripts/check_package_comments.sh
./scripts/check_package_comments.sh
```

---

### 6. 导出函数注释检查脚本

**文件**: `scripts/check_exported_comments.sh`

```bash
#!/bin/bash
# 检查缺少注释的导出函数

echo "检查缺少注释的导出函数..."

find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" ! -name "*_test.go" | while read file; do
    # 查找导出函数（以大写字母开头）
    grep -n "^func ([A-Za-z].*) [A-Z]" "$file" | while read line; do
        linenum=$(echo "$line" | cut -d: -f1)
        # 检查前一行是否有注释
        prevline=$((linenum - 1))
        if ! sed -n "${prevline}p" "$file" | grep -q "^//"; then
            echo "⚠️  $file:$linenum - 缺少导出函数注释"
        fi
    done
done
```

**使用方法**:
```bash
chmod +x scripts/check_exported_comments.sh
./scripts/check_exported_comments.sh
```

---

### 7. 敏感信息检查脚本

**文件**: `scripts/check_sensitive_logs.sh`

```bash
#!/bin/bash
# 检查日志中的敏感信息

echo "检查日志中的敏感信息..."

# 检查token
grep -r "token" --include="*.go" . | grep -i "logrus\|log\|fmt.Print" | grep -v "// " | while read line; do
    echo "⚠️  可能的token泄露: $line"
done

# 检查password
grep -r "password" --include="*.go" . | grep -i "logrus\|log\|fmt.Print" | grep -v "// " | while read line; do
    echo "⚠️  可能的password泄露: $line"
done

# 检查secret
grep -r "secret" --include="*.go" . | grep -i "logrus\|log\|fmt.Print" | grep -v "// " | while read line; do
    echo "⚠️  可能的secret泄露: $line"
done
```

**使用方法**:
```bash
chmod +x scripts/check_sensitive_logs.sh
./scripts/check_sensitive_logs.sh
```

---

## 🔧 静态分析工具配置

### 1. golangci-lint配置

**文件**: `.golangci.yml`

```yaml
# golangci-lint配置文件
run:
  timeout: 5m
  skip-dirs:
    - vendor
    - .git
  skip-files:
    - ".*_test.go"

linters:
  enable:
    - errcheck        # 检查未处理的错误
    - govet           # 检查常见错误
    - staticcheck     # 静态分析
    - gosec           # 安全检查
    - gocritic        # 代码质量检查
    - gofmt           # 格式检查
    - goimports       # import检查
    - misspell        # 拼写检查
    - ineffassign     # 无效赋值检查
    - unconvert       # 不必要的类型转换
    - unused          # 未使用的代码
    - deadcode        # 死代码检查

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gosec
        - govet
    - path: cmd/
      linters:
        - gosec

output:
  format: colored-line-number
  print-issued-lines: true
  print-linter-name: true
```

**使用方法**:
```bash
# 安装
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 运行
golangci-lint run ./...

# 修复自动可修复的问题
golangci-lint run --fix ./...
```

---

### 2. Go Vet配置

**文件**: `scripts/run_go_vet.sh`

```bash
#!/bin/bash
# 运行Go Vet进行代码检查

echo "运行Go Vet..."

go vet ./...

if [ $? -eq 0 ]; then
    echo "✅ Go Vet检查通过"
else
    echo "❌ Go Vet检查失败"
    exit 1
fi
```

**使用方法**:
```bash
chmod +x scripts/run_go_vet.sh
./scripts/run_go_vet.sh
```

---

## 📋 综合检查脚本

### 1. 完整的最佳实践检查脚本

**文件**: `scripts/check_all_best_practices.sh`

```bash
#!/bin/bash
# 完整的Go最佳实践检查脚本

set -e

echo "=========================================="
echo "Go 最佳实践全面检查"
echo "=========================================="
echo ""

# 1. 检查文件长度
echo "1️⃣  检查文件长度..."
./scripts/check_file_length.sh
echo ""

# 2. 检查Goroutine panic recovery
echo "2️⃣  检查Goroutine panic recovery..."
./scripts/fix_goroutine_panic.sh
echo ""

# 3. 检查Context使用
echo "3️⃣  检查Context使用..."
./scripts/check_context_usage.sh
echo ""

# 4. 检查包注释
echo "4️⃣  检查包注释..."
./scripts/check_package_comments.sh
echo ""

# 5. 检查导出函数注释
echo "5️⃣  检查导出函数注释..."
./scripts/check_exported_comments.sh
echo ""

# 6. 检查敏感信息
echo "6️⃣  检查敏感信息..."
./scripts/check_sensitive_logs.sh
echo ""

# 7. 运行Go Vet
echo "7️⃣  运行Go Vet..."
go vet ./...
echo ""

# 8. 运行golangci-lint
echo "8️⃣  运行golangci-lint..."
golangci-lint run ./...
echo ""

echo "=========================================="
echo "✅ 检查完成"
echo "=========================================="
```

**使用方法**:
```bash
chmod +x scripts/check_all_best_practices.sh
./scripts/check_all_best_practices.sh
```

---

## 🔄 Git Pre-commit Hook

### 1. Pre-commit Hook配置

**文件**: `.git/hooks/pre-commit`

```bash
#!/bin/sh
# Git pre-commit hook - 在提交前运行最佳实践检查

set -e

echo "运行Go最佳实践检查..."

# 获取暂存的Go文件
STAGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)

if [ -z "$STAGED_GO_FILES" ]; then
    echo "没有Go文件需要检查"
    exit 0
fi

echo "检查以下文件:"
echo "$STAGED_GO_FILES"
echo ""

# 1. 运行gofmt
echo "1️⃣  运行gofmt..."
for file in $STAGED_GO_FILES; do
    gofmt -w "$file"
    git add "$file"
done
echo "✅ gofmt检查完成"
echo ""

# 2. 运行go vet
echo "2️⃣  运行go vet..."
go vet ./...
echo "✅ go vet检查完成"
echo ""

# 3. 运行golangci-lint
echo "3️⃣  运行golangci-lint..."
golangci-lint run $STAGED_GO_FILES
echo "✅ golangci-lint检查完成"
echo ""

# 4. 运行测试
echo "4️⃣  运行测试..."
go test ./...
echo "✅ 测试通过"
echo ""

echo "✅ 所有检查通过，可以提交"
exit 0
```

**安装方法**:
```bash
# 创建hooks目录
mkdir -p .git/hooks

# 复制pre-commit文件
cp .git/hooks/pre-commit .git/hooks/pre-commit

# 添加执行权限
chmod +x .git/hooks/pre-commit
```

---

## 📊 修复进度跟踪

### 1. 修复进度表

**文件**: `BEST_PRACTICES_PROGRESS.md`

```markdown
# Go 最佳实践修复进度

## 修复统计

| 问题类型 | 总数 | 已修复 | 进度 | 优先级 |
|---------|------|--------|------|--------|
| 文件长度超过300行 | 42 | 0 | 0% | 🔴 |
| Goroutine缺少Panic Recovery | 15 | 7 | 47% | 🔴 |
| 错误处理使用%v而不是%w | 25 | 0 | 0% | 🔴 |
| Context使用不当 | 30+ | 0 | 0% | 🔴 |
| 日志输出敏感信息 | 8 | 0 | 0% | 🟠 |
| 缺少导出函数注释 | 150+ | 0 | 0% | 🟠 |
| 缺少包注释 | 80+ | 0 | 0% | 🟠 |
| Goroutine退出条件不完善 | 12 | 0 | 0% | 🟠 |
| 切片容量预分配不足 | 20+ | 0 | 0% | 🟡 |
| Context作为结构体字段 | 3 | 0 | 0% | 🟡 |

## 修复计划

### 第一周（优先级最高）
- [ ] 修复所有Goroutine panic recovery问题
- [ ] 修复所有错误处理%v->%w问题
- [ ] 修复Context使用不当问题

### 第二周（优先级高）
- [ ] 移除日志中的敏感信息
- [ ] 添加所有缺少的包注释
- [ ] 添加所有缺少的导出函数注释

### 第三周（优先级中）
- [ ] 完善Goroutine退出条件
- [ ] 优化切片初始化
- [ ] 移除Context字段

### 第四周及以后（优先级低）
- [ ] 拆分超过300行的文件
- [ ] 改进变量命名

## 修复记录

### 2024-XX-XX
- 修复了X个Goroutine panic recovery问题
- 修复了X个错误处理问题
- ...

```

---

## 🎯 修复建议总结

### 快速修复（1-2小时）
```bash
# 1. 修复错误处理
./scripts/fix_error_handling.sh

# 2. 检查Goroutine
./scripts/fix_goroutine_panic.sh

# 3. 检查敏感信息
./scripts/check_sensitive_logs.sh
```

### 中期修复（1-2周）
```bash
# 1. 检查包注释
./scripts/check_package_comments.sh

# 2. 检查导出函数注释
./scripts/check_exported_comments.sh

# 3. 检查Context使用
./scripts/check_context_usage.sh
```

### 长期改进（2-4周）
```bash
# 1. 检查文件长度
./scripts/check_file_length.sh

# 2. 运行完整检查
./scripts/check_all_best_practices.sh

# 3. 运行静态分析
golangci-lint run ./...
```

---

## 📚 参考资源

### 官方文档
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Best Practices](https://golang.org/doc/effective_go)

### 工具
- [golangci-lint](https://golangci-lint.run/)
- [Go Vet](https://golang.org/cmd/vet/)
- [gofmt](https://golang.org/cmd/gofmt/)

### 相关项目
- [uber-go/guide](https://github.com/uber-go/guide)
- [golang-standards/project-layout](https://github.com/golang-standards/project-layout)

