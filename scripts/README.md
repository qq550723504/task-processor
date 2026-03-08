# scripts 目录

## 用途

存放项目相关的脚本文件，包括部署脚本、测试脚本、数据处理脚本等。

## 目录结构

```
scripts/
├── deploy-windows.ps1      # Windows 部署脚本
├── purge-queue.py          # 清空队列脚本
├── send-test-message.py    # 发送测试消息脚本
├── build.sh                # 构建脚本（建议添加）
├── test.sh                 # 测试脚本（建议添加）
└── migrate.sh              # 数据迁移脚本（建议添加）
```

## 应该放置的文件

- 部署脚本（Shell/PowerShell/Python）
- 构建脚本
- 测试脚本
- 数据库迁移脚本
- 数据处理脚本
- 运维工具脚本
- CI/CD 相关脚本

## 脚本命名规范

1. 使用小写字母和连字符：`deploy-production.sh`
2. 使用描述性动词开头：`build-`, `deploy-`, `test-`
3. 根据平台选择合适的扩展名：
   - Linux/Mac: `.sh`
   - Windows: `.ps1` (PowerShell) 或 `.bat`
   - Python: `.py`

## 脚本编写规范

### Shell 脚本示例

```bash
#!/bin/bash
set -e  # 遇到错误立即退出

# 脚本说明
# 用途：构建项目
# 用法：./build.sh [环境]

ENVIRONMENT=${1:-dev}

echo "开始构建 ${ENVIRONMENT} 环境..."

# 清理旧的构建文件
rm -rf bin/*

# 构建
go build -o bin/task-processor ./cmd/task

echo "构建完成！"
```

### PowerShell 脚本示例

```powershell
# 脚本说明
# 用途：部署到 Windows 服务器
# 用法：.\deploy-windows.ps1 -Environment prod

param(
    [string]$Environment = "dev"
)

Write-Host "开始部署到 $Environment 环境..."

# 停止服务
Stop-Service -Name "TaskProcessor" -ErrorAction SilentlyContinue

# 复制文件
Copy-Item -Path "bin\*" -Destination "C:\Services\TaskProcessor\" -Recurse -Force

# 启动服务
Start-Service -Name "TaskProcessor"

Write-Host "部署完成！"
```

### Python 脚本示例

```python
#!/usr/bin/env python3
"""
清空 RabbitMQ 队列
用法：python purge-queue.py --queue task-queue
"""

import argparse
import pika

def purge_queue(queue_name):
    """清空指定队列"""
    connection = pika.BlockingConnection(
        pika.ConnectionParameters('localhost')
    )
    channel = connection.channel()
    channel.queue_purge(queue_name)
    connection.close()
    print(f"队列 {queue_name} 已清空")

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='清空 RabbitMQ 队列')
    parser.add_argument('--queue', required=True, help='队列名称')
    args = parser.parse_args()
    
    purge_queue(args.queue)
```

## 注意事项

- 脚本开头添加 shebang（`#!/bin/bash` 或 `#!/usr/bin/env python3`）
- 添加详细的注释说明脚本用途和用法
- 使用参数化，避免硬编码
- 添加错误处理和日志输出
- 设置适当的文件权限（`chmod +x script.sh`）
- 敏感信息使用环境变量
- 提供使用示例和帮助信息
