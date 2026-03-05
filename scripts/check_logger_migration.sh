#!/bin/bash

# 日志迁移检查脚本
# 用于检查项目中需要迁移的日志代码

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 统计变量
total_issues=0

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}日志迁移检查工具${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 1. 检查直接使用 logrus 全局方法
echo -e "${YELLOW}[1] 检查直接使用 logrus 全局方法...${NC}"
logrus_global=$(grep -rn "logrus\.\(Info\|Error\|Warn\|Debug\|Fatal\|Panic\)f\?" --include="*.go" . 2>/dev/null | \
    grep -v "vendor" | grep -v ".git" | grep -v "_test.go" | grep -v "// " || true)

if [ -n "$logrus_global" ]; then
    count=$(echo "$logrus_global" | wc -l)
    total_issues=$((total_issues + count))
    echo -e "${RED}发现 $count 处使用 logrus 全局方法：${NC}"
    echo "$logrus_global" | head -20
    if [ $(echo "$logrus_global" | wc -l) -gt 20 ]; then
        echo -e "${YELLOW}... (仅显示前20条)${NC}"
    fi
else
    echo -e "${GREEN}✓ 未发现使用 logrus 全局方法${NC}"
fi
echo ""

# 2. 检查使用 fmt.Printf 调试输出
echo -e "${YELLOW}[2] 检查使用 fmt.Printf 调试输出...${NC}"
fmt_printf=$(grep -rn "fmt\.Printf\|fmt\.Println" --include="*.go" . 2>/dev/null | \
    grep -v "vendor" | grep -v ".git" | grep -v "_test.go" | grep -v "examples_test.go" | grep -v "// " || true)

if [ -n "$fmt_printf" ]; then
    count=$(echo "$fmt_printf" | wc -l)
    total_issues=$((total_issues + count))
    echo -e "${RED}发现 $count 处使用 fmt.Printf：${NC}"
    echo "$fmt_printf" | head -20
    if [ $(echo "$fmt_printf" | wc -l) -gt 20 ]; then
        echo -e "${YELLOW}... (仅显示前20条)${NC}"
    fi
else
    echo -e "${GREEN}✓ 未发现使用 fmt.Printf${NC}"
fi
echo ""

# 3. 检查字段命名不规范
echo -e "${YELLOW}[3] 检查字段命名不规范（应使用 snake_case）...${NC}"
field_naming=$(grep -rn "\"taskId\"\|\"TaskID\"\|\"productId\"\|\"ProductID\"\|\"tenantId\"\|\"TenantID\"" --include="*.go" . 2>/dev/null | \
    grep -v "vendor" | grep -v ".git" | grep -v "// " || true)

if [ -n "$field_naming" ]; then
    count=$(echo "$field_naming" | wc -l)
    total_issues=$((total_issues + count))
    echo -e "${RED}发现 $count 处字段命名不规范：${NC}"
    echo "$field_naming" | head -20
    if [ $(echo "$field_naming" | wc -l) -gt 20 ]; then
        echo -e "${YELLOW}... (仅显示前20条)${NC}"
    fi
else
    echo -e "${GREEN}✓ 未发现字段命名不规范${NC}"
fi
echo ""

# 4. 检查是否导入了新的 logger 包
echo -e "${YELLOW}[4] 检查是否导入了新的 logger 包...${NC}"
logger_import=$(grep -rn "\"task-processor/internal/core/logger\"" --include="*.go" . 2>/dev/null | \
    grep -v "vendor" | grep -v ".git" | wc -l || echo "0")

go_files=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./*_test.go" | wc -l)

echo -e "${BLUE}已导入新 logger 包的文件: $logger_import / $go_files${NC}"
if [ "$logger_import" -lt "$((go_files / 2))" ]; then
    echo -e "${YELLOW}⚠ 大部分文件尚未迁移到新的 logger 包${NC}"
else
    echo -e "${GREEN}✓ 大部分文件已迁移到新的 logger 包${NC}"
fi
echo ""

# 5. 检查是否使用了标准字段常量
echo -e "${YELLOW}[5] 检查是否使用了标准字段常量...${NC}"
field_constants=$(grep -rn "logger\.Field\(Component\|Platform\|TaskID\|ProductID\)" --include="*.go" . 2>/dev/null | \
    grep -v "vendor" | grep -v ".git" | wc -l || echo "0")

if [ "$field_constants" -gt 0 ]; then
    echo -e "${GREEN}✓ 发现 $field_constants 处使用了标准字段常量${NC}"
else
    echo -e "${YELLOW}⚠ 未发现使用标准字段常量，建议使用 logger.Field* 常量${NC}"
fi
echo ""

# 6. 检查是否有重复创建 logger 的情况
echo -e "${YELLOW}[6] 检查是否有重复创建 logger...${NC}"
new_logger=$(grep -rn "logrus\.New()" --include="*.go" . 2>/dev/null | \
    grep -v "vendor" | grep -v ".git" | grep -v "_test.go" || true)

if [ -n "$new_logger" ]; then
    count=$(echo "$new_logger" | wc -l)
    total_issues=$((total_issues + count))
    echo -e "${RED}发现 $count 处创建新的 logger 实例：${NC}"
    echo "$new_logger"
else
    echo -e "${GREEN}✓ 未发现重复创建 logger${NC}"
fi
echo ""

# 7. 检查是否使用了 WithError
echo -e "${YELLOW}[7] 检查错误日志是否使用 WithError...${NC}"
error_logs=$(grep -rn "\.Error\|\.Errorf" --include="*.go" . 2>/dev/null | \
    grep -v "vendor" | grep -v ".git" | grep -v "_test.go" | wc -l || echo "0")
with_error=$(grep -rn "\.WithError(" --include="*.go" . 2>/dev/null | \
    grep -v "vendor" | grep -v ".git" | grep -v "_test.go" | wc -l || echo "0")

if [ "$error_logs" -gt 0 ]; then
    ratio=$((with_error * 100 / error_logs))
    echo -e "${BLUE}错误日志总数: $error_logs, 使用 WithError: $with_error (${ratio}%)${NC}"
    if [ "$ratio" -lt 50 ]; then
        echo -e "${YELLOW}⚠ 建议更多地使用 WithError() 方法记录错误${NC}"
    else
        echo -e "${GREEN}✓ 大部分错误日志使用了 WithError()${NC}"
    fi
else
    echo -e "${GREEN}✓ 未发现错误日志${NC}"
fi
echo ""

# 总结
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}检查总结${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}发现的问题总数: ${RED}$total_issues${NC}"
echo ""

if [ "$total_issues" -eq 0 ]; then
    echo -e "${GREEN}✓ 恭喜！所有检查都通过了！${NC}"
    exit 0
else
    echo -e "${YELLOW}建议：${NC}"
    echo -e "1. 参考 docs/日志迁移指南.md 进行迁移"
    echo -e "2. 参考 docs/日志迁移示例.md 查看具体示例"
    echo -e "3. 使用 logger.GetGlobalLogger() 替代 logrus 全局方法"
    echo -e "4. 使用结构化日志字段替代 fmt.Printf"
    echo -e "5. 使用 logger.Field* 常量统一字段命名"
    echo ""
    exit 1
fi
