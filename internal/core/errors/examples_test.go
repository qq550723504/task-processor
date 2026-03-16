package errors_test

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/core/errors"
)

// 示例：基本错误创建和使用
func ExampleNew() {
	err := errors.New(errors.ErrCodeValidation, "产品ID不能为空")
	fmt.Println(err)
	// Output: [VALIDATION_ERROR] 产品ID不能为空
}

// 示例：包装现有错误
func ExampleWrap() {
	originalErr := fmt.Errorf("connection refused")
	err := errors.Wrap(originalErr, errors.ErrCodeNetwork, "连接数据库失败")
	fmt.Println(err)
	// Output: [NETWORK_ERROR] 连接数据库失败: connection refused
}

// 示例：格式化错误消息
func ExampleWrapf() {
	originalErr := fmt.Errorf("not found")
	productID := "12345"
	err := errors.Wrapf(originalErr, errors.ErrCodeTaskNotFound, "产品不存在: productID=%s", productID)
	fmt.Println(err)
	// Output: [TASK_NOT_FOUND] 产品不存在: productID=12345: not found
}

// 示例：判断错误类型
func ExampleIsCode() {
	err := errors.New(errors.ErrCodeNetwork, "网络错误")

	if errors.IsCode(err, errors.ErrCodeNetwork) {
		fmt.Println("这是一个网络错误")
	}
	// Output: 这是一个网络错误
}

// 示例：判断是否可重试
func ExampleIsRetryable() {
	networkErr := errors.New(errors.ErrCodeNetwork, "网络超时")
	validationErr := errors.New(errors.ErrCodeValidation, "数据验证失败")

	fmt.Printf("网络错误可重试: %v\n", errors.IsRetryable(networkErr))
	fmt.Printf("验证错误可重试: %v\n", errors.IsRetryable(validationErr))
	// Output:
	// 网络错误可重试: true
	// 验证错误可重试: false
}

// 示例：使用重试机制
func ExampleRetry() {
	ctx := context.Background()
	config := errors.DefaultRetryConfig()
	config.MaxRetries = 3
	config.InitialDelay = 100 * time.Millisecond

	attempts := 0
	err := errors.Retry(ctx, config, func() error {
		attempts++
		if attempts < 3 {
			return errors.New(errors.ErrCodeNetwork, "临时网络错误")
		}
		return nil
	})

	if err == nil {
		fmt.Printf("成功，共尝试 %d 次\n", attempts)
	}
	// Output: 成功，共尝试 3 次
}

// 示例：合并多个错误
func ExampleCombine() {
	err1 := errors.New(errors.ErrCodeValidation, "字段A验证失败")
	err2 := errors.New(errors.ErrCodeValidation, "字段B验证失败")
	err3 := errors.New(errors.ErrCodeValidation, "字段C验证失败")

	combinedErr := errors.Combine(err1, err2, err3)
	fmt.Println(combinedErr)
	// Output: multiple errors (3):
	//   1. [VALIDATION_ERROR] 字段A验证失败
	//   2. [VALIDATION_ERROR] 字段B验证失败
	//   3. [VALIDATION_ERROR] 字段C验证失败
}

// 示例：忽略特定错误
func ExampleIgnoreError() {
	err := errors.New(errors.ErrCodeValidation, "验证失败")

	// 忽略验证错误
	result := errors.IgnoreError(err, errors.ErrCodeValidation)
	fmt.Printf("忽略后的错误: %v\n", result)
	// Output: 忽略后的错误: <nil>
}

// mockLogger 用于示例
type mockLogger struct{}

func (m *mockLogger) Error(args ...any)                 {}
func (m *mockLogger) Errorf(format string, args ...any) {}
func (m *mockLogger) Warn(args ...any)                  {}
func (m *mockLogger) Warnf(format string, args ...any)  {}
func (m *mockLogger) Info(args ...any)                  {}
func (m *mockLogger) Infof(format string, args ...any)  {}

// 示例：使用错误处理器
func ExampleDefaultErrorHandler() {
	logger := &mockLogger{}
	handler := errors.NewDefaultErrorHandler(logger)

	// 处理网络错误
	networkErr := errors.New(errors.ErrCodeNetwork, "网络超时")
	handler.Handle(networkErr)

	if handler.ShouldRetry(networkErr) {
		fmt.Println("网络错误应该重试")
	}

	// 处理关键错误
	systemErr := errors.New(errors.ErrCodeSystem, "系统崩溃")
	handler.Handle(systemErr)

	if handler.ShouldTerminate(systemErr) {
		fmt.Println("系统错误应该终止")
	}

	// Output:
	// 网络错误应该重试
	// 系统错误应该终止
}

// 示例：安全执行函数
func ExampleSafeExecute() {
	logger := &mockLogger{}

	err := errors.SafeExecute(func() error {
		// 模拟可能panic的代码
		// panic("something went wrong")
		return nil
	}, logger)

	if err == nil {
		fmt.Println("执行成功")
	}
	// Output: 执行成功
}

// 示例：实际业务场景 - 产品服务
type ProductService struct {
	errorHandler errors.ErrorHandler
}

func NewProductService(logger errors.Logger) *ProductService {
	return &ProductService{
		errorHandler: errors.NewDefaultErrorHandler(logger),
	}
}

func (s *ProductService) GetProduct(id string) error {
	// 验证输入
	if id == "" {
		return errors.New(errors.ErrCodeValidation, "产品ID不能为空")
	}

	// 模拟数据库查询
	err := s.queryDatabase(id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeExternalAPI, "查询产品失败")
	}

	return nil
}

func (s *ProductService) queryDatabase(id string) error {
	// 模拟数据库错误
	return fmt.Errorf("database connection failed")
}

func ExampleProductService() {
	logger := &mockLogger{}
	service := NewProductService(logger)

	// 测试空ID
	err := service.GetProduct("")
	if errors.IsCode(err, errors.ErrCodeValidation) {
		fmt.Println("验证错误：产品ID为空")
	}

	// 测试数据库错误
	err = service.GetProduct("12345")
	if errors.IsCode(err, errors.ErrCodeExternalAPI) {
		fmt.Println("外部API错误：查询失败")
	}

	// Output:
	// 验证错误：产品ID为空
	// 外部API错误：查询失败
}

// 示例：实际业务场景 - 带重试的API客户端
type APIClient struct {
	logger errors.Logger
}

func (c *APIClient) FetchData(ctx context.Context, url string) error {
	config := errors.DefaultRetryConfig()
	config.MaxRetries = 3
	config.InitialDelay = time.Second

	return errors.Retry(ctx, config, func() error {
		// 模拟API调用
		err := c.callAPI(url)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeNetwork, "API调用失败")
		}
		return nil
	})
}

func (c *APIClient) callAPI(url string) error {
	// 模拟网络错误
	return fmt.Errorf("connection timeout")
}

func ExampleAPIClient() {
	logger := &mockLogger{}
	client := &APIClient{logger: logger}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.FetchData(ctx, "https://api.example.com/data")
	if err != nil {
		fmt.Println("API调用失败，已重试多次")
	}
	// Output: API调用失败，已重试多次
}
