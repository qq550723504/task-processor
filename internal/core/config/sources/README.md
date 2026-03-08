# 配置源实现

## 目录说明

此目录用于存放各种配置源的实现。

## 设计目的

支持从多种来源加载配置,实现`ConfigSource`接口:
- 文件配置源(已在`source.go`中实现)
- 环境变量配置源
- 远程配置中心(如Consul、etcd)
- 数据库配置源
- HTTP配置源

## ConfigSource接口

```go
type ConfigSource interface {
    // Read 读取配置数据
    Read() ([]byte, error)
    
    // Watch 监听配置变化
    Watch(ctx context.Context, callback func([]byte)) error
    
    // Name 返回配置源名称
    Name() string
}
```

## 使用示例

### 环境变量配置源

```go
package sources

type EnvConfigSource struct {
    prefix string
}

func NewEnvConfigSource(prefix string) *EnvConfigSource {
    return &EnvConfigSource{prefix: prefix}
}

func (e *EnvConfigSource) Read() ([]byte, error) {
    // 从环境变量读取配置
    // 转换为YAML或JSON格式
}

func (e *EnvConfigSource) Watch(ctx context.Context, callback func([]byte)) error {
    // 环境变量通常不支持监听
    return nil
}

func (e *EnvConfigSource) Name() string {
    return fmt.Sprintf("env:%s", e.prefix)
}
```

### Consul配置源

```go
package sources

type ConsulConfigSource struct {
    client *consul.Client
    key    string
}

func NewConsulConfigSource(addr, key string) (*ConsulConfigSource, error) {
    // 创建Consul客户端
    // 返回配置源
}

func (c *ConsulConfigSource) Read() ([]byte, error) {
    // 从Consul读取配置
}

func (c *ConsulConfigSource) Watch(ctx context.Context, callback func([]byte)) error {
    // 监听Consul配置变化
}
```

## 当前状态

🚧 此目录当前为空,预留用于未来扩展。

基础的文件配置源已在`../source.go`中实现。如果需要支持其他配置源,可以在此目录下实现。

## 注意事项

- 所有配置源实现应该是线程安全的
- Watch方法应该在context取消时正确清理资源
- 配置源应该处理好错误情况,避免panic
