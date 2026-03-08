# cmd 目录

## 用途

存放项目的所有可执行程序入口文件（main.go）。每个子目录代表一个独立的可执行程序。

## 目录结构

```
cmd/
├── amazon-crawler/      # Amazon 爬虫独立程序
├── crawler-consumer/    # 爬虫消费者程序
├── rabbitmq-consumer/   # RabbitMQ 消息消费者
├── task/                # 主任务处理器程序
├── test-1688/           # 1688 平台测试程序
└── test-product-fetcher/ # 产品获取测试程序
```

## 应该放置的文件

- `main.go` - 程序入口文件
- 程序特定的初始化代码
- 命令行参数解析
- 配置加载逻辑

## 编码规范

1. 每个子目录只包含一个 `main.go` 文件
2. main 函数应该尽量简洁，主要负责：
   - 解析命令行参数
   - 加载配置
   - 初始化依赖
   - 启动应用
3. 业务逻辑应该放在 `internal/` 目录下
4. 使用依赖注入，避免在 main 中硬编码依赖关系

## 示例

```go
package main

import (
    "context"
    "flag"
    "log"
    
    "task-processor/internal/infra/bootstrap"
)

func main() {
    // 解析命令行参数
    configPath := flag.String("config", "config/config.yaml", "配置文件路径")
    flag.Parse()
    
    // 初始化应用
    app := bootstrap.NewApplicationBootstrap(logger)
    if err := app.Initialize(*configPath, version); err != nil {
        log.Fatal(err)
    }
    
    // 启动应用
    ctx := context.Background()
    if err := app.Start(ctx, version); err != nil {
        log.Fatal(err)
    }
}
```

## 注意事项

- 不要在 cmd 目录中放置可复用的业务逻辑
- 每个程序应该有清晰的职责边界
- 使用 `-ldflags` 注入版本信息等构建时变量
