# domain 目录

## 用途

领域层，包含业务领域模型、业务规则和领域服务。这一层定义了业务的核心概念和规则，不依赖具体的技术实现。

## 目录结构

```
domain/
├── model/       # 领域模型（实体、值对象）
├── product/     # 产品领域
├── task/        # 任务领域
└── validation/  # 业务验证规则
```

## 子目录说明

### model（领域模型）
- 通用的领域实体
- 值对象定义
- 领域事件

**应该放置的文件：**
- `entity.go` - 实体基类
- `value_object.go` - 值对象
- `event.go` - 领域事件

### product（产品领域）
- 产品相关的领域模型
- 产品业务规则
- 产品领域服务

**应该放置的文件：**
- `product.go` - 产品实体
- `category.go` - 分类值对象
- `price.go` - 价格值对象
- `service.go` - 产品领域服务
- `repository.go` - 产品仓储接口

### task（任务领域）
- 任务相关的领域模型
- 任务状态机
- 任务业务规则

**应该放置的文件：**
- `task.go` - 任务实体
- `status.go` - 任务状态
- `priority.go` - 任务优先级
- `service.go` - 任务领域服务
- `repository.go` - 任务仓储接口

### validation（业务验证）
- 业务规则验证
- 敏感词检查
- 禁止项检查
- 数据完整性验证

**应该放置的文件：**
- `validator.go` - 验证器接口
- `product_validator.go` - 产品验证器
- `sensitive_word_checker.go` - 敏感词检查器
- `prohibited_item_checker.go` - 禁止项检查器

## 领域驱动设计原则

### 1. 实体（Entity）
- 有唯一标识
- 有生命周期
- 可变的

```go
type Product struct {
    ID          string
    Title       string
    Price       Money
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 2. 值对象（Value Object）
- 无唯一标识
- 不可变
- 通过属性值判断相等性

```go
type Money struct {
    Amount   float64
    Currency string
}

func (m Money) Equals(other Money) bool {
    return m.Amount == other.Amount && m.Currency == other.Currency
}
```

### 3. 聚合根（Aggregate Root）
- 管理聚合内的一致性
- 对外提供统一的访问入口

```go
type Order struct {
    ID          string
    Items       []OrderItem
    TotalAmount Money
}

func (o *Order) AddItem(item OrderItem) error {
    // 业务规则验证
    o.Items = append(o.Items, item)
    o.recalculateTotal()
    return nil
}
```

### 4. 领域服务（Domain Service）
- 不属于任何实体的业务逻辑
- 协调多个实体的操作

```go
type ProductService interface {
    ValidateProduct(product *Product) error
    CalculateShippingCost(product *Product, destination string) (Money, error)
}
```

### 5. 仓储接口（Repository）
- 定义数据访问接口
- 具体实现在 infra 层

```go
type ProductRepository interface {
    Save(product *Product) error
    FindByID(id string) (*Product, error)
    FindAll() ([]*Product, error)
}
```

## 编码规范

1. 领域模型应该是纯粹的业务逻辑，不包含技术细节
2. 使用充血模型，将业务逻辑封装在实体内部
3. 值对象应该是不可变的
4. 领域服务处理跨实体的业务逻辑
5. 仓储接口在 domain 层定义，在 infra 层实现

## 示例代码

```go
// product/product.go
package product

import "errors"

// Product 产品实体
type Product struct {
    id          string
    title       string
    price       Money
    category    Category
    status      Status
}

// NewProduct 创建产品
func NewProduct(id, title string, price Money, category Category) (*Product, error) {
    if title == "" {
        return nil, errors.New("产品标题不能为空")
    }
    if price.Amount <= 0 {
        return nil, errors.New("产品价格必须大于0")
    }
    
    return &Product{
        id:       id,
        title:    title,
        price:    price,
        category: category,
        status:   StatusDraft,
    }, nil
}

// Publish 发布产品
func (p *Product) Publish() error {
    if p.status == StatusPublished {
        return errors.New("产品已发布")
    }
    p.status = StatusPublished
    return nil
}

// UpdatePrice 更新价格
func (p *Product) UpdatePrice(newPrice Money) error {
    if newPrice.Amount <= 0 {
        return errors.New("价格必须大于0")
    }
    p.price = newPrice
    return nil
}
```

## 注意事项

- 领域层不依赖基础设施层
- 业务规则应该在领域层实现
- 保持领域模型的纯粹性
- 使用领域语言命名
- 充分的单元测试
