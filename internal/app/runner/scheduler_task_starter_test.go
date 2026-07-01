package runner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/listingruntime"

	"github.com/sirupsen/logrus"
)

type stubSchedulerStoreRuntime struct {
	getStoreFunc                 func(storeID int64) (*listingruntime.StoreInfo, error)
	listAutoPricingStoreIDsFunc  func(ctx context.Context, platformName string) ([]int64, error)
	listScheduledTaskConfigsFunc func(ctx context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error)
}

func (s *stubSchedulerStoreRuntime) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	if s.getStoreFunc != nil {
		return s.getStoreFunc(storeID)
	}
	return &listingruntime.StoreInfo{ID: storeID}, nil
}

func (s *stubSchedulerStoreRuntime) ListAutoPricingStoreIDs(ctx context.Context, platformName string) ([]int64, error) {
	if s.listAutoPricingStoreIDsFunc == nil {
		return nil, fmt.Errorf("unexpected ListAutoPricingStoreIDs call")
	}
	return s.listAutoPricingStoreIDsFunc(ctx, platformName)
}

func (s *stubSchedulerStoreRuntime) ListScheduledTaskConfigs(ctx context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
	if s.listScheduledTaskConfigsFunc == nil {
		return nil, nil
	}
	return s.listScheduledTaskConfigsFunc(ctx, platformName, taskType)
}

func TestResolveStoreIDsForTaskDiscoversAutoPricingStores(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	calls := 0
	storeRuntime := &stubSchedulerStoreRuntime{
		listAutoPricingStoreIDsFunc: func(_ context.Context, platformName string) ([]int64, error) {
			calls++
			if platformName != "SHEIN" {
				t.Fatalf("expected platform SHEIN, got %s", platformName)
			}
			return []int64{10, 8, 10}, nil
		},
	}

	storeIDs := resolveStoreIDsForTask("SHEIN", scheduler.TaskTypePricing, nil, storeRuntime, logger)
	expected := []int64{8, 10}
	if len(storeIDs) != len(expected) {
		t.Fatalf("expected %d store IDs, got %d", len(expected), len(storeIDs))
	}
	for i := range expected {
		if storeIDs[i] != expected[i] {
			t.Fatalf("expected storeIDs[%d]=%d, got %d", i, expected[i], storeIDs[i])
		}
	}
	if calls != 1 {
		t.Fatalf("expected 1 PageStores call, got %d", calls)
	}
}

func TestResolveStoreIDsForTaskSkipsDynamicDiscoveryForNonPricingTasks(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	storeRuntime := &stubSchedulerStoreRuntime{
		listAutoPricingStoreIDsFunc: func(_ context.Context, _ string) ([]int64, error) {
			t.Fatalf("ListAutoPricingStoreIDs should not be called for non-pricing tasks")
			return nil, nil
		},
	}

	storeIDs := resolveStoreIDsForTask("TEMU", scheduler.TaskTypeInventory, nil, storeRuntime, logger)
	if len(storeIDs) != 0 {
		t.Fatalf("expected no store IDs, got %v", storeIDs)
	}
}

func TestResolveStoreIDsForTaskUsesExplicitStoreIDsForInventoryTasks(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	storeRuntime := &stubSchedulerStoreRuntime{
		listAutoPricingStoreIDsFunc: func(_ context.Context, _ string) ([]int64, error) {
			t.Fatalf("ListAutoPricingStoreIDs should not be called when explicit inventory stores are configured")
			return nil, nil
		},
	}

	storeIDs := resolveStoreIDsForTask("SHEIN", scheduler.TaskTypeInventory, []int64{398, 181, 398, 0}, storeRuntime, logger)
	expected := []int64{181, 398}
	if len(storeIDs) != len(expected) {
		t.Fatalf("expected %d store IDs, got %d: %v", len(expected), len(storeIDs), storeIDs)
	}
	for i := range expected {
		if storeIDs[i] != expected[i] {
			t.Fatalf("expected storeIDs[%d]=%d, got %d", i, expected[i], storeIDs[i])
		}
	}
}

func TestResolveStoreTaskConfigsIncludesAdminEnabledInventoryStores(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	storeRuntime := &stubSchedulerStoreRuntime{
		listScheduledTaskConfigsFunc: func(_ context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
			if platformName != "SHEIN" {
				t.Fatalf("expected platform SHEIN, got %s", platformName)
			}
			if taskType != scheduler.TaskTypeInventory {
				t.Fatalf("expected inventory task type, got %s", taskType)
			}
			return []listingruntime.ScheduledTaskConfig{
				{StoreID: 962, Platform: "shein", TaskType: "inventory", Enabled: true, IntervalSeconds: 1800},
				{StoreID: 963, Platform: "SHEIN", TaskType: "inventory", Enabled: true, IntervalSeconds: 0},
			}, nil
		},
	}

	configs := resolveStoreTaskConfigs("SHEIN", scheduler.TaskTypeInventory, nil, time.Hour, storeRuntime, logger)
	expectedStores := []int64{962, 963}
	if len(configs) != len(expectedStores) {
		t.Fatalf("expected %d store configs, got %d: %+v", len(expectedStores), len(configs), configs)
	}
	for i, storeID := range expectedStores {
		if configs[i].StoreID != storeID {
			t.Fatalf("expected configs[%d].StoreID=%d, got %d", i, storeID, configs[i].StoreID)
		}
	}
	if configs[0].Interval != 30*time.Minute {
		t.Fatalf("expected first interval 30m, got %s", configs[0].Interval)
	}
	if configs[1].Interval != time.Hour {
		t.Fatalf("expected default interval for missing DB interval, got %s", configs[1].Interval)
	}
}

func TestResolveStoreTaskConfigsAdminConfigOverridesExplicitInterval(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	storeRuntime := &stubSchedulerStoreRuntime{
		listScheduledTaskConfigsFunc: func(_ context.Context, _ string, _ scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
			return []listingruntime.ScheduledTaskConfig{
				{StoreID: 962, Platform: "shein", TaskType: "inventory", Enabled: true, IntervalSeconds: 900},
			}, nil
		},
	}

	configs := resolveStoreTaskConfigs("SHEIN", scheduler.TaskTypeInventory, []int64{962, 964}, time.Hour, storeRuntime, logger)
	if len(configs) != 2 {
		t.Fatalf("expected 2 store configs, got %d: %+v", len(configs), configs)
	}
	if configs[0].StoreID != 962 || configs[0].Interval != 15*time.Minute {
		t.Fatalf("expected store 962 interval from admin config, got %+v", configs[0])
	}
	if configs[1].StoreID != 964 || configs[1].Interval != time.Hour {
		t.Fatalf("expected store 964 interval from static config, got %+v", configs[1])
	}
}

func TestShouldStartTasksByTypeUsesAdminScheduledConfigWhenStaticConfigDisabled(t *testing.T) {
	t.Parallel()

	calls := 0
	service := &schedulerServiceImpl{
		logger: logrus.New(),
		storeRuntime: &stubSchedulerStoreRuntime{
			listScheduledTaskConfigsFunc: func(_ context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
				calls++
				if platformName != "SHEIN" || taskType != scheduler.TaskTypeInventory {
					t.Fatalf("unexpected scheduled config query %s/%s", platformName, taskType)
				}
				return []listingruntime.ScheduledTaskConfig{
					{StoreID: 962, Platform: "shein", TaskType: "inventory", Enabled: true, IntervalSeconds: 3600},
				}, nil
			},
		},
	}

	if !service.shouldStartTasksByType("SHEIN", scheduler.TaskTypeInventory, taskTypeConfig{Enabled: false}) {
		t.Fatal("expected admin scheduled config to enable inventory startup")
	}
	if calls != 1 {
		t.Fatalf("expected 1 scheduled config lookup, got %d", calls)
	}
}

func TestSyncEnabledScheduledTaskConfigsStartsRuntimeAddedConfig(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &schedulerServiceImpl{
		logger: logrus.New(),
		config: &config.Config{},
		storeRuntime: &stubSchedulerStoreRuntime{
			getStoreFunc: func(storeID int64) (*listingruntime.StoreInfo, error) {
				return &listingruntime.StoreInfo{
					ID:       storeID,
					TenantID: 227,
					Platform: "shein",
				}, nil
			},
			listScheduledTaskConfigsFunc: func(_ context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
				if platformName == "SHEIN" && taskType == scheduler.TaskTypeProductSync {
					return []listingruntime.ScheduledTaskConfig{
						{StoreID: 870, Platform: "shein", TaskType: "productSync", Enabled: true, IntervalSeconds: 3600},
					}, nil
				}
				return nil, nil
			},
		},
		sheinFactoryCreator: func(*config.Config) scheduler.TaskFactory {
			return stubRuntimeScheduledTaskFactory{platform: "shein"}
		},
		ctx: ctx,
	}
	service.schedulerManager = scheduler.NewManager(ctx, time.Second)

	created, err := service.syncEnabledScheduledTaskConfigs(ctx)
	if err != nil {
		t.Fatalf("syncEnabledScheduledTaskConfigs() error = %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 created task, got %d", created)
	}
	tasks := service.schedulerManager.ListTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 scheduler task, got %d", len(tasks))
	}
	if tasks[0].GetStoreID() != 870 || tasks[0].GetType() != scheduler.TaskTypeProductSync {
		t.Fatalf("unexpected task: id=%s store=%d type=%s", tasks[0].GetID(), tasks[0].GetStoreID(), tasks[0].GetType())
	}

	created, err = service.syncEnabledScheduledTaskConfigs(ctx)
	if err != nil {
		t.Fatalf("second syncEnabledScheduledTaskConfigs() error = %v", err)
	}
	if created != 0 {
		t.Fatalf("expected second sync to create 0 tasks, got %d", created)
	}
	if got := service.schedulerManager.GetTaskCount(); got != 1 {
		t.Fatalf("expected scheduler task count to remain 1, got %d", got)
	}
}

type stubRuntimeScheduledTaskFactory struct {
	platform string
}

func (f stubRuntimeScheduledTaskFactory) CreateTask(_ context.Context, config scheduler.TaskConfig) (scheduler.Task, error) {
	return stubRuntimeScheduledTask{config: config}, nil
}

func (f stubRuntimeScheduledTaskFactory) SupportedPlatform() string { return f.platform }

func (f stubRuntimeScheduledTaskFactory) SupportedTaskTypes() []scheduler.TaskType {
	return []scheduler.TaskType{
		scheduler.TaskTypePricing,
		scheduler.TaskTypeProductSync,
		scheduler.TaskTypeInventory,
		scheduler.TaskTypeActivity,
	}
}

type stubRuntimeScheduledTask struct {
	config scheduler.TaskConfig
}

func (t stubRuntimeScheduledTask) GetID() string {
	return fmt.Sprintf("%s:%s:%d:%d", t.config.Platform, t.config.TaskType, t.config.TenantID, t.config.StoreID)
}

func (t stubRuntimeScheduledTask) GetType() scheduler.TaskType { return t.config.TaskType }

func (t stubRuntimeScheduledTask) GetPlatform() string { return t.config.Platform }

func (t stubRuntimeScheduledTask) GetStoreID() int64 { return t.config.StoreID }

func (t stubRuntimeScheduledTask) Execute(context.Context) error { return nil }

func (t stubRuntimeScheduledTask) GetInterval() time.Duration { return t.config.Interval }

func (t stubRuntimeScheduledTask) GetStatus() scheduler.TaskStatus {
	return scheduler.TaskStatusStopped
}
