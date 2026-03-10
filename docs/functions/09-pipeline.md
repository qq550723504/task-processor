# 函数清单 - Pipeline 模块

生成时间: 2026-03-10

## pipeline 模块

### pipeline.go
```go
func NewPipeline(name string) Pipeline
func (p *BasePipeline) AddHandler(handler Handler) Pipeline
func (p *BasePipeline) Process(ctx TaskContext) error
func (p *BasePipeline) GetHandlerCount() int
func (p *BasePipeline) GetName() string
func (p *BasePipeline) Clear()
```

### base_handler.go
```go
func NewBaseHandler(name string) *BaseHandler
func (bh *BaseHandler) Name() string
func (bh *BaseHandler) GetLogger() *logrus.Entry
func (bh *BaseHandler) ValidateContext(ctx TaskContext) error
func (bh *BaseHandler) GetRequiredData(ctx TaskContext, key string) (interface{}, error)
func (bh *BaseHandler) SetData(ctx TaskContext, key string, value interface{})
```

### parallel_handler.go
```go
func NewParallelHandler(name string, handlers ...Handler) *ParallelHandler
func (h *ParallelHandler) Name() string
func (h *ParallelHandler) Handle(ctx TaskContext) error
func (h *ParallelHandler) AddHandler(handler Handler)
func (h *ParallelHandler) GetHandlers() []Handler
```

### context_impl.go
```go
func NewTaskContext(ctx context.Context, task *model.Task) TaskContext
func NewTemuTaskContext(ctx context.Context, task *model.Task) TemuTaskContext
func NewSheinTaskContext(ctx context.Context, task *model.Task) SheinTaskContext
func (tc *DefaultTaskContext) GetContext() context.Context
func (tc *DefaultTaskContext) GetTask() *model.Task
func (tc *DefaultTaskContext) SetData(key string, value interface{})
func (tc *DefaultTaskContext) GetData(key string) (interface{}, bool)
func (tc *DefaultTaskContext) GetLogger() *logrus.Entry
```

### errors.go
```go
func NewHandlerError(handlerName, message string) *HandlerError
func NewHandlerErrorWithCause(handlerName, message string, cause error) *HandlerError
func (e *HandlerError) Error() string
func (e *HandlerError) Unwrap() error
func NewPipelineError(pipelineName, message string) *PipelineError
func (e *PipelineError) Error() string
```

## pipeline/handlers 模块

### init_handler.go
```go
func NewInitHandler() pipeline.Handler
func (h *InitHandler) Handle(ctx pipeline.TaskContext) error
func (h *InitHandler) Name() string
```

### logging_handler.go
```go
func NewLoggingHandler(logLevel string) pipeline.Handler
func (h *LoggingHandler) Handle(ctx pipeline.TaskContext) error
func (h *LoggingHandler) Name() string
func (h *LoggingHandler) SetLogLevel(level string)
```

### validation_handler.go
```go
func NewValidationHandler(validators ...Validator) pipeline.Handler
func (h *ValidationHandler) Handle(ctx pipeline.TaskContext) error
func (h *ValidationHandler) Name() string
func (h *ValidationHandler) AddValidator(validator Validator)
func NewTaskValidator() Validator
func (v *TaskValidator) Name() string
func (v *TaskValidator) Validate(ctx pipeline.TaskContext) error
```

### retry_handler.go
```go
func NewRetryHandler(maxRetries int, backoff time.Duration) pipeline.Handler
func (h *RetryHandler) Handle(ctx pipeline.TaskContext) error
func (h *RetryHandler) Name() string
func (h *RetryHandler) SetMaxRetries(maxRetries int)
func (h *RetryHandler) SetBackoff(backoff time.Duration)
```

### timeout_handler.go
```go
func NewTimeoutHandler(timeout time.Duration) pipeline.Handler
func (h *TimeoutHandler) Handle(ctx pipeline.TaskContext) error
func (h *TimeoutHandler) Name() string
func (h *TimeoutHandler) SetTimeout(timeout time.Duration)
```
