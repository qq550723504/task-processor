# Mock 对象目录

## 用途

存放测试用的 Mock 对象，用于隔离测试和模拟外部依赖。

## 目录结构

```
mocks/
├── README.md              # 本文件
├── mock_services.go       # 服务 Mock
├── mock_repositories.go   # 仓储 Mock
├── mock_clients.go        # 客户端 Mock
└── mock_processors.go     # 处理器 Mock
```

## Mock 对象示例

### mock_services.go

```go
package mocks

import (
    "context"
    "github.com/stretchr/testify/mock"
)

// MockUserService 用户服务 Mock
type MockUserService struct {
    mock.Mock
}

func (m *MockUserService) GetUserByID(ctx context.Context, id int64) (*User, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserService) CreateUser(ctx context.Context, user *User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}

// MockTaskService 任务服务 Mock
type MockTaskService struct {
    mock.Mock
}

func (m *MockTaskService) ProcessTask(ctx context.Context, taskID int64) error {
    args := m.Called(ctx, taskID)
    return args.Error(0)
}

func (m *MockTaskService) GetTaskStatus(ctx context.Context, taskID int64) (string, error) {
    args := m.Called(ctx, taskID)
    return args.String(0), args.Error(1)
}
```

### mock_repositories.go

```go
package mocks

import (
    "context"
    "github.com/stretchr/testify/mock"
)

// MockProductRepository 产品仓储 Mock
type MockProductRepository struct {
    mock.Mock
}

func (m *MockProductRepository) Save(ctx context.Context, product *Product) error {
    args := m.Called(ctx, product)
    return args.Error(0)
}

func (m *MockProductRepository) FindByID(ctx context.Context, id string) (*Product, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*Product), args.Error(1)
}

func (m *MockProductRepository) FindAll(ctx context.Context) ([]*Product, error) {
    args := m.Called(ctx)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]*Product), args.Error(1)
}

// MockTaskRepository 任务仓储 Mock
type MockTaskRepository struct {
    mock.Mock
}

func (m *MockTaskRepository) Save(ctx context.Context, task *Task) error {
    args := m.Called(ctx, task)
    return args.Error(0)
}

func (m *MockTaskRepository) FindByID(ctx context.Context, id int64) (*Task, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*Task), args.Error(1)
}
```

### mock_clients.go

```go
package mocks

import (
    "context"
    "github.com/stretchr/testify/mock"
)

// MockManagementClient 管理系统客户端 Mock
type MockManagementClient struct {
    mock.Mock
}

func (m *MockManagementClient) GetTasks(ctx context.Context, platform string) ([]*Task, error) {
    args := m.Called(ctx, platform)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]*Task), args.Error(1)
}

func (m *MockManagementClient) ReportResult(ctx context.Context, taskID int64, result *TaskResult) error {
    args := m.Called(ctx, taskID, result)
    return args.Error(0)
}

// MockCrawlerClient 爬虫客户端 Mock
type MockCrawlerClient struct {
    mock.Mock
}

func (m *MockCrawlerClient) FetchProduct(ctx context.Context, url string) (*Product, error) {
    args := m.Called(ctx, url)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*Product), args.Error(1)
}
```

## 使用方法

### 1. 基本使用

```go
package mypackage

import (
    "context"
    "testing"
    "task-processor/tests/mocks"
    "github.com/stretchr/testify/assert"
)

func TestProcessTask(t *testing.T) {
    // 创建 Mock 对象
    mockTaskService := new(mocks.MockTaskService)
    mockProductRepo := new(mocks.MockProductRepository)
    
    // 设置期望
    ctx := context.Background()
    taskID := int64(123)
    
    mockTaskService.On("ProcessTask", ctx, taskID).Return(nil)
    mockProductRepo.On("FindByID", ctx, "product-1").Return(&Product{
        ID:    "product-1",
        Title: "Test Product",
    }, nil)
    
    // 执行测试
    err := mockTaskService.ProcessTask(ctx, taskID)
    product, err := mockProductRepo.FindByID(ctx, "product-1")
    
    // 断言
    assert.NoError(t, err)
    assert.NotNil(t, product)
    assert.Equal(t, "Test Product", product.Title)
    
    // 验证 Mock 调用
    mockTaskService.AssertExpectations(t)
    mockProductRepo.AssertExpectations(t)
}
```

### 2. 模拟错误情况

```go
func TestProcessTaskError(t *testing.T) {
    mockTaskService := new(mocks.MockTaskService)
    ctx := context.Background()
    taskID := int64(123)
    
    // 模拟错误
    expectedError := errors.New("task not found")
    mockTaskService.On("ProcessTask", ctx, taskID).Return(expectedError)
    
    // 执行测试
    err := mockTaskService.ProcessTask(ctx, taskID)
    
    // 断言错误
    assert.Error(t, err)
    assert.Equal(t, expectedError, err)
    
    mockTaskService.AssertExpectations(t)
}
```

### 3. 模拟多次调用

```go
func TestMultipleCalls(t *testing.T) {
    mockRepo := new(mocks.MockProductRepository)
    ctx := context.Background()
    
    // 第一次调用返回产品
    mockRepo.On("FindByID", ctx, "product-1").Return(&Product{
        ID: "product-1",
    }, nil).Once()
    
    // 第二次调用返回错误
    mockRepo.On("FindByID", ctx, "product-1").Return(nil, errors.New("not found")).Once()
    
    // 执行测试
    product1, err1 := mockRepo.FindByID(ctx, "product-1")
    assert.NoError(t, err1)
    assert.NotNil(t, product1)
    
    product2, err2 := mockRepo.FindByID(ctx, "product-1")
    assert.Error(t, err2)
    assert.Nil(t, product2)
    
    mockRepo.AssertExpectations(t)
}
```

### 4. 使用 AnythingOfType

```go
func TestWithAnyType(t *testing.T) {
    mockRepo := new(mocks.MockProductRepository)
    ctx := context.Background()
    
    // 接受任何 Product 类型的参数
    mockRepo.On("Save", ctx, mock.AnythingOfType("*Product")).Return(nil)
    
    // 执行测试
    err := mockRepo.Save(ctx, &Product{ID: "any-id"})
    
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

## 生成 Mock 对象

### 使用 mockery 工具

安装：
```bash
go install github.com/vektra/mockery/v2@latest
```

生成 Mock：
```bash
# 为单个接口生成 Mock
mockery --name=UserService --output=tests/mocks

# 为整个包生成 Mock
mockery --dir=internal/domain --all --output=tests/mocks

# 使用配置文件
mockery --config=.mockery.yaml
```

配置文件 `.mockery.yaml`:
```yaml
with-expecter: true
dir: "internal"
output: "tests/mocks"
outpkg: "mocks"
packages:
  task-processor/internal/domain:
    interfaces:
      UserService:
      ProductRepository:
      TaskService:
```

## 最佳实践

1. **命名规范**
   - Mock 类型以 `Mock` 开头
   - 文件名以 `mock_` 开头

2. **接口隔离**
   - 为每个接口创建独立的 Mock
   - 保持 Mock 简单

3. **验证调用**
   - 使用 `AssertExpectations` 验证
   - 使用 `AssertCalled` 检查特定调用

4. **清理**
   - 在测试结束时验证 Mock
   - 避免 Mock 对象泄漏

5. **文档化**
   - 为复杂的 Mock 添加注释
   - 说明 Mock 的用途

## 注意事项

1. 不要过度使用 Mock
2. Mock 应该反映真实行为
3. 定期更新 Mock 以匹配接口变化
4. 考虑使用真实对象进行集成测试
5. Mock 不能替代端到端测试
