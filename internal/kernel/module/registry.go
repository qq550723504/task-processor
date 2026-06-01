package module

import (
	"fmt"
	"reflect"
	"strings"

	"task-processor/internal/httproute"
	"task-processor/internal/infra/worker"
)

type NamedWorkerPool struct {
	Name string
	Pool worker.WorkerPool
}

type NamedTemporalWorker struct {
	Name  string
	Start TemporalWorkerStarter
}

type Registry struct {
	routes              []httproute.Descriptor
	workerPools         []NamedWorkerPool
	workerPoolNames     map[string]struct{}
	temporalWorkers     []NamedTemporalWorker
	temporalWorkerNames map[string]struct{}
	taskHandlers        map[string]TaskHandler
	workflowNames       map[string]struct{}
}

type WorkflowRegistry struct {
	names      map[string]struct{}
	expected   string
	registered bool
}

func NewRegistry() *Registry {
	return &Registry{
		workerPoolNames:     make(map[string]struct{}),
		temporalWorkerNames: make(map[string]struct{}),
		taskHandlers:        make(map[string]TaskHandler),
		workflowNames:       make(map[string]struct{}),
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

func (r *Registry) AddWorkerPool(name string, pool worker.WorkerPool) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("worker pool name is empty")
	}
	if isNilInterface(pool) {
		return fmt.Errorf("worker pool is nil: %s", name)
	}
	if _, exists := r.workerPoolNames[name]; exists {
		return fmt.Errorf("worker pool already registered: %s", name)
	}

	r.workerPoolNames[name] = struct{}{}
	r.workerPools = append(r.workerPools, NamedWorkerPool{Name: name, Pool: pool})
	return nil
}

func (r *Registry) WorkerPools() []NamedWorkerPool {
	out := make([]NamedWorkerPool, len(r.workerPools))
	copy(out, r.workerPools)
	return out
}

func (r *Registry) AddTemporalWorker(name string, start TemporalWorkerStarter) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("temporal worker name is empty")
	}
	if isNilInterface(start) {
		return fmt.Errorf("temporal worker starter is nil: %s", name)
	}
	if _, exists := r.temporalWorkerNames[name]; exists {
		return fmt.Errorf("temporal worker already registered: %s", name)
	}

	r.temporalWorkerNames[name] = struct{}{}
	r.temporalWorkers = append(r.temporalWorkers, NamedTemporalWorker{Name: name, Start: start})
	return nil
}

func (r *Registry) TemporalWorkers() []NamedTemporalWorker {
	out := make([]NamedTemporalWorker, len(r.temporalWorkers))
	copy(out, r.temporalWorkers)
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
