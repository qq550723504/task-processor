package module

import (
	"fmt"
	"reflect"

	"task-processor/internal/httproute"
)

type Registry struct {
	routes        []httproute.Descriptor
	taskHandlers  map[string]TaskHandler
	workflowNames map[string]struct{}
}

type WorkflowRegistry struct {
	names      map[string]struct{}
	expected   string
	registered bool
}

func NewRegistry() *Registry {
	return &Registry{
		taskHandlers:  make(map[string]TaskHandler),
		workflowNames: make(map[string]struct{}),
	}
}

func (r *Registry) AddRoutes(routes ...httproute.Descriptor) {
	r.routes = append(r.routes, routes...)
}

func (r *Registry) Routes() []httproute.Descriptor {
	out := make([]httproute.Descriptor, len(r.routes))
	copy(out, r.routes)
	return out
}

func (r *Registry) RegisterTaskHandler(handler TaskHandler) error {
	if isNilInterface(handler) {
		return fmt.Errorf("task handler is nil")
	}

	taskType := handler.TaskType()
	if _, exists := r.taskHandlers[taskType]; exists {
		return fmt.Errorf("task handler already registered: %s", taskType)
	}

	r.taskHandlers[taskType] = handler
	return nil
}

func (r *Registry) RegisterWorkflowHandler(handler WorkflowHandler) error {
	if isNilInterface(handler) {
		return fmt.Errorf("workflow handler is nil")
	}

	expected := handler.WorkflowName()
	reg := &WorkflowRegistry{
		names:    r.workflowNames,
		expected: expected,
	}

	if err := handler.RegisterWorkflow(reg); err != nil {
		return err
	}
	if !reg.registered {
		return fmt.Errorf("workflow handler did not register workflow: %s", expected)
	}

	return nil
}

func (r *WorkflowRegistry) Register(name string) error {
	if name != r.expected {
		return fmt.Errorf("workflow handler registered unexpected name: got %s want %s", name, r.expected)
	}
	if _, exists := r.names[name]; exists {
		return fmt.Errorf("workflow handler already registered: %s", name)
	}

	r.names[name] = struct{}{}
	r.registered = true
	return nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
