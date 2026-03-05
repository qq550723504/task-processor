#!/bin/bash

# 自动日志迁移脚本
# 用于批量迁移文件中的日志代码

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}自动日志迁移工具${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查参数
if [ "$#" -lt 1 ]; then
    echo -e "${RED}用法: $0 <目录或文件>${NC}"
    echo "示例:"
    echo "  $0 internal/platforms/temu/handlers"
    echo "  $0 internal/platforms/temu/handlers/image_handler.go"
    exit 1
fi

TARGET=$1
BACKUP_DIR=".migration_backup_$(date +%Y%m%d_%H%M%S)"

# 创建备份目录
mkdir -p "$BACKUP_DIR"
echo -e "${YELLOW}备份目录: $BACKUP_DIR${NC}"
echo ""

# 查找需要迁移的文件
if [ -f "$TARGET" ]; then
    FILES=("$TARGET")
elif [ -d "$TARGET" ]; then
    FILES=($(find "$TARGET" -name "*.go" -type f | grep -v "_test.go"))
else
    echo -e "${RED}错误: $TARGET 不是有效的文件或目录${NC}"
    exit 1
fi

echo -e "${BLUE}找到 ${#FILES[@]} 个文件${NC}"
echo ""

MIGRATED=0
SKIPPED=0
FAILED=0

for file in "${FILES[@]}"; do
    echo -e "${YELLOW}处理: $file${NC}"
    
    # 检查是否已经导入了logger包
    if grep -q "task-processor/internal/core/logger" "$file"; then
        echo -e "${GREEN}  ✓ 已迁移，跳过${NC}"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi
    
    # 检查是否使用了logrus
    if ! grep -q "logrus\." "$file"; then
        echo -e "${YELLOW}  - 未使用logrus，跳过${NC}"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi
    
    # 备份原文件
    cp "$file" "$BACKUP_DIR/"
    
    # 创建临时文件
    TEMP_FILE="${file}.tmp"
    
    # 1. 添加logger导入（如果还没有）
    if ! grep -q "task-processor/internal/core/logger" "$file"; then
        # 在import块中添加logger导入
        sed -i '/import (/a\	"task-processor/internal/core/logger"' "$file"
    fi
    
    # 2. 替换logrus全局调用为logger.GetGlobalLogger
    # logrus.Info -> log.Info (需要先获取logger)
    # logrus.Infof -> log.WithFields().Info
    
    # 3. 替换字段名
    sed -i 's/"taskId"/"task_id"/g' "$file"
    sed -i 's/"TaskID"/"task_id"/g' "$file"
    sed -i 's/"productId"/"product_id"/g' "$file"
    sed -i 's/"ProductID"/"product_id"/g' "$file"
    sed -i 's/"tenantId"/"tenant_id"/g' "$file"
    sed -i 's/"TenantID"/"tenant_id"/g' "$file"
    sed -i 's/"storeId"/"store_id"/g' "$file"
    sed -i 's/"StoreID"/"store_id"/g' "$file"
    
    # 验证编译
    if go build "$file" 2>/dev/null; then
        echo -e "${GREEN}  ✓ 迁移成功${NC}"
        MIGRATED=$((MIGRATED + 1))
    else
        echo -e "${RED}  ✗ 编译失败，恢复原文件${NC}"
        cp "$BACKUP_DIR/$(basename $file)" "$file"
        FAILED=$((FAILED + 1))
    fi
    
    echo ""
done

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}迁移完成${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}成功: $MIGRATED${NC}"
echo -e "${YELLOW}跳过: $SKIPPED${NC}"
echo -e "${RED}失败: $FAILED${NC}"
echo ""
echo -e "${YELLOW}备份位置: $BACKUP_DIR${NC}"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}部分文件迁移失败，请手动检查${NC}"
    exit 1
fi

echo -e "${GREEN}所有文件迁移成功！${NC}"
