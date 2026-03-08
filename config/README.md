# config 目录

## 用途

存放应用程序的配置文件，支持不同环境的配置管理。

## 目录结构

```
config/
├── config-dev.yaml      # 开发环境配置
├── config-prod.yaml     # 生产环境配置（如果有）
├── config-test.yaml     # 测试环境配置（如果有）
└── rabbitmq-config.yaml # RabbitMQ 专用配置
```

## 应该放置的文件

- YAML/JSON/TOML 格式的配置文件
- 环境特定的配置文件
- 配置模板文件
- 配置文件示例（.example 后缀）

## 配置文件命名规范

1. 使用 `config-{环境}.yaml` 格式命名主配置文件
2. 特定组件的配置可以单独文件：`{组件名}-config.yaml`
3. 敏感配置使用环境变量或 `.env` 文件（不提交到版本控制）

## 配置文件结构示例

```yaml
# config-dev.yaml
app:
  name: task-processor
  version: 1.0.0
  
server:
  host: 0.0.0.0
  port: 8080
  
database:
  host: localhost
  port: 5432
  name: taskdb
  
platforms:
  temu:
    enabled: true
    workers: 5
  shein:
    enabled: true
    workers: 3
    
rabbitmq:
  host: localhost
  port: 5672
  username: guest
  password: guest
```

## 注意事项

- 不要将敏感信息（密码、密钥）直接写入配置文件
- 使用 `.gitignore` 忽略包含敏感信息的配置文件
- 提供 `.example` 文件作为配置模板
- 配置文件应该有清晰的注释说明
- 使用环境变量覆盖配置文件中的敏感值
