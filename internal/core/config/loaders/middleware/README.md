# 配置加载器中间件

## 目录说明

此目录用于存放配置加载器的中间件实现。

## 设计目的

提供配置加载的中间件机制,支持在配置加载过程中插入自定义逻辑:
- 配置验证中间件
- 配置转换中间件
- 配置加密/解密中间件
- 配置审计中间件
- 配置缓存中间件

## 中间件接口设计

```go
package middleware

// LoaderMiddleware 配置加载器中间件
type LoaderMiddleware interface {
    // Process 处理配置数据
    // 返回处理后的数据和可能的错误
    Process(data []byte, next func([]byte) ([]byte, error)) ([]byte, error)
}

// MiddlewareFunc 中间件函数类型
type MiddlewareFunc func([]byte, func([]byte) ([]byte, error)) ([]byte, error)

// Chain 中间件链
type Chain struct {
    middlewares []LoaderMiddleware
}

func NewChain(middlewares ...LoaderMiddleware) *Chain {
    return &Chain{middlewares: middlewares}
}

func (c *Chain) Execute(data []byte) ([]byte, error) {
    // 执行中间件链
}
```

## 使用示例

### 配置解密中间件

```go
package middleware

type DecryptionMiddleware struct {
    key []byte
}

func NewDecryptionMiddleware(key []byte) *DecryptionMiddleware {
    return &DecryptionMiddleware{key: key}
}

func (d *DecryptionMiddleware) Process(data []byte, next func([]byte) ([]byte, error)) ([]byte, error) {
    // 解密配置数据
    decrypted, err := decrypt(data, d.key)
    if err != nil {
        return nil, err
    }
    
    // 传递给下一个中间件
    return next(decrypted)
}
```

### 配置验证中间件

```go
package middleware

type ValidationMiddleware struct {
    schema string
}

func NewValidationMiddleware(schema string) *ValidationMiddleware {
    return &ValidationMiddleware{schema: schema}
}

func (v *ValidationMiddleware) Process(data []byte, next func([]byte) ([]byte, error)) ([]byte, error) {
    // 验证配置格式
    if err := validateAgainstSchema(data, v.schema); err != nil {
        return nil, err
    }
    
    // 传递给下一个中间件
    return next(data)
}
```

### 使用中间件链

```go
// 创建中间件链
chain := middleware.NewChain(
    middleware.NewDecryptionMiddleware(key),
    middleware.NewValidationMiddleware(schema),
    middleware.NewAuditMiddleware(logger),
)

// 在加载器中使用
data, err := chain.Execute(rawData)
if err != nil {
    return nil, err
}
```

## 当前状态

🚧 此目录当前为空,预留用于未来扩展。

如果需要在配置加载过程中添加复杂的处理逻辑,可以在此目录下实现中间件。

## 设计原则

- 中间件应该是可组合的
- 中间件应该是无状态的(或线程安全的)
- 中间件应该有清晰的职责
- 中间件应该能够优雅地处理错误
- 中间件的执行顺序应该是可控的
