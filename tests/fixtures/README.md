# 测试数据目录

## 用途

存放测试所需的固定数据（fixtures），包括测试用的 JSON 文件、配置文件等。

## 目录结构

```
fixtures/
├── README.md           # 本文件
├── products.json       # 产品测试数据
├── tasks.json          # 任务测试数据
├── configs/            # 测试配置文件
│   ├── test-config.yaml
│   └── minimal-config.yaml
└── responses/          # API 响应数据
    ├── success.json
    └── error.json
```

## 文件说明

### products.json

产品测试数据示例：

```json
{
  "products": [
    {
      "id": "test-product-1",
      "title": "Test Product 1",
      "price": 99.99,
      "currency": "USD",
      "platform": "temu",
      "status": "active"
    },
    {
      "id": "test-product-2",
      "title": "Test Product 2",
      "price": 149.99,
      "currency": "USD",
      "platform": "shein",
      "status": "active"
    }
  ]
}
```

### tasks.json

任务测试数据示例：

```json
{
  "tasks": [
    {
      "id": 1,
      "platform": "temu",
      "action": "product_sync",
      "status": "pending",
      "payload": {
        "product_id": "test-product-1",
        "store_id": 1
      }
    },
    {
      "id": 2,
      "platform": "shein",
      "action": "inventory_check",
      "status": "processing",
      "payload": {
        "product_ids": ["test-product-2", "test-product-3"],
        "store_id": 2
      }
    }
  ]
}
```

### configs/test-config.yaml

测试配置文件示例：

```yaml
app:
  name: task-processor-test
  
platforms:
  temu:
    enabled: true
    workers: 1
  shein:
    enabled: false
    
worker:
  task_interval: 5
  max_workers: 2
  
browser:
  enabled: false
```

### responses/success.json

成功响应示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": 123,
    "status": "completed"
  }
}
```

### responses/error.json

错误响应示例：

```json
{
  "code": 400,
  "message": "invalid request",
  "data": null
}
```

## 使用方法

### 在测试中加载 fixtures

```go
package mypackage

import (
    "encoding/json"
    "os"
    "testing"
)

func loadProductFixture(t *testing.T) []Product {
    data, err := os.ReadFile("fixtures/products.json")
    if err != nil {
        t.Fatal(err)
    }
    
    var fixture struct {
        Products []Product `json:"products"`
    }
    
    if err := json.Unmarshal(data, &fixture); err != nil {
        t.Fatal(err)
    }
    
    return fixture.Products
}

func TestProductProcessing(t *testing.T) {
    products := loadProductFixture(t)
    
    for _, product := range products {
        // 使用测试数据
        result := processProduct(product)
        // 断言...
    }
}
```

### 使用辅助函数

创建 `fixtures/loader.go`:

```go
package fixtures

import (
    "encoding/json"
    "os"
    "path/filepath"
)

// LoadJSON 加载 JSON fixture
func LoadJSON(filename string, v interface{}) error {
    path := filepath.Join("fixtures", filename)
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, v)
}

// LoadProducts 加载产品数据
func LoadProducts() ([]Product, error) {
    var fixture struct {
        Products []Product `json:"products"`
    }
    
    if err := LoadJSON("products.json", &fixture); err != nil {
        return nil, err
    }
    
    return fixture.Products, nil
}

// LoadTasks 加载任务数据
func LoadTasks() ([]Task, error) {
    var fixture struct {
        Tasks []Task `json:"tasks"`
    }
    
    if err := LoadJSON("tasks.json", &fixture); err != nil {
        return nil, err
    }
    
    return fixture.Tasks, nil
}
```

在测试中使用：

```go
import "task-processor/tests/fixtures"

func TestWithFixtures(t *testing.T) {
    products, err := fixtures.LoadProducts()
    if err != nil {
        t.Fatal(err)
    }
    
    // 使用产品数据
}
```

## 最佳实践

1. **保持数据最小化**
   - 只包含测试所需的最少数据
   - 避免大型数据文件

2. **使用有意义的数据**
   - 数据应该反映真实场景
   - 使用清晰的命名

3. **版本控制**
   - 将 fixtures 提交到版本控制
   - 记录数据变更原因

4. **文档化**
   - 说明每个 fixture 的用途
   - 提供使用示例

5. **独立性**
   - 测试不应该修改 fixture 数据
   - 每个测试应该独立运行

## 注意事项

1. 不要在 fixtures 中包含敏感信息
2. 使用相对路径加载文件
3. 考虑使用 `testdata` 目录（Go 的约定）
4. 大型文件考虑压缩存储
5. 定期清理不再使用的 fixtures
