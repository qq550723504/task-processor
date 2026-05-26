package consumer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"task-processor/internal/core/config"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

type noopRabbitHandler struct{}

func (noopRabbitHandler) HandleMessage(_ context.Context, _ *rabbitmq.Message) error {
	return nil
}

type noopProcessor struct{}

func (noopProcessor) Start(_ context.Context) error { return nil }

func (noopProcessor) ProcessTask(_ context.Context, _ worker.WorkerJob) error { return nil }

func (noopProcessor) Close(_ context.Context) {}

type countingStoreAPI struct {
	stubStoreAPI
	calls []int64
}

func (s *countingStoreAPI) GetStore(id int64) (*managementapi.StoreRespDTO, error) {
	s.calls = append(s.calls, id)
	return s.stubStoreAPI.GetStore(id)
}

type stubStoreAssignmentProvider struct {
	stores []int64
	err    error
}

func (p stubStoreAssignmentProvider) GetOwnedStores(_ context.Context, _ string) ([]int64, error) {
	if p.err != nil {
		return nil, p.err
	}
	return append([]int64(nil), p.stores...), nil
}

func (p stubStoreAssignmentProvider) Close() error { return nil }

func TestRabbitMQServiceFilterQueueConfigsByRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		expected []string
	}{
		{
			name:     "task role keeps only task queues",
			role:     config.NodeRoleTask,
			expected: []string{"amazon.tasks.store.*", "shein.tasks", "shein.tasks.bucket.*", "shein.tasks.store.*"},
		},
		{
			name:     "crawler role keeps only crawler queues",
			role:     config.NodeRoleCrawler,
			expected: []string{"amazon.crawler", "1688.crawler"},
		},
		{
			name:     "hybrid role keeps all queues",
			role:     config.NodeRoleHybrid,
			expected: []string{"amazon.tasks.store.*", "amazon.crawler", "1688.crawler", "shein.tasks", "shein.tasks.bucket.*", "shein.tasks.store.*"},
		},
	}

	source := []config.QueueConfig{
		{Name: "amazon.tasks.store.*"},
		{Name: "amazon.crawler"},
		{Name: "1688.crawler"},
		{Name: "shein.tasks"},
		{Name: "shein.tasks.bucket.*"},
		{Name: "shein.tasks.store.*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewRabbitMQService(&config.RabbitMQConfig{
				URL: "amqp://guest:guest@localhost:5672/",
				Node: config.NodeConfig{
					Role: tt.role,
				},
			}, logrus.New())

			filtered := svc.filterQueueConfigsByRole(source)
			if len(filtered) != len(tt.expected) {
				t.Fatalf("expected %d queue configs, got %d", len(tt.expected), len(filtered))
			}

			for i, expected := range tt.expected {
				if filtered[i].Name != expected {
					t.Fatalf("expected queue %q at index %d, got %q", expected, i, filtered[i].Name)
				}
			}
		})
	}
}

func TestRabbitMQServiceRegistersSheinBucketHandlers(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())

	svc.processorRegistry.RegisterProcessor("shein", noopProcessor{})
	svc.registerMessageHandlers()

	if svc.GetConsumer().GetStateManager("shein.tasks") == nil {
		t.Fatal("expected shared shein queue handler to be registered")
	}

	for bucket := 0; bucket < sheinBucketQueueCount; bucket++ {
		queueName := fmt.Sprintf("shein.tasks.bucket.%d", bucket)
		if svc.GetConsumer().GetStateManager(queueName) == nil {
			t.Fatalf("expected shein bucket handler %s to be registered", queueName)
		}
	}
}

func TestRabbitMQServiceRegistersOwnedSheinBucketsOnly(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:         config.NodeRoleTask,
			OwnedBuckets: []int{5, 2, 2, 9, -1},
		},
	}, logrus.New())

	svc.processorRegistry.RegisterProcessor("shein", noopProcessor{})
	svc.registerMessageHandlers()

	if svc.GetConsumer().GetStateManager("shein.tasks") == nil {
		t.Fatal("expected shared shein queue handler to stay registered")
	}

	for _, bucket := range []int{2, 5} {
		queueName := fmt.Sprintf("shein.tasks.bucket.%d", bucket)
		if svc.GetConsumer().GetStateManager(queueName) == nil {
			t.Fatalf("expected shein owned bucket handler %s to be registered", queueName)
		}
	}

	for _, bucket := range []int{0, 1, 3, 4, 6, 7} {
		queueName := fmt.Sprintf("shein.tasks.bucket.%d", bucket)
		if svc.GetConsumer().GetStateManager(queueName) != nil {
			t.Fatalf("did not expect shein bucket handler %s to be registered", queueName)
		}
	}
}

func TestRabbitMQServiceDoesNotFallbackToSharedQueuesWhenUsingStoreQueuesWithoutAssignments(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:           config.NodeRoleTask,
			UseStoreQueues: true,
		},
	}, logrus.New())

	svc.processorRegistry.RegisterProcessor("shein", noopProcessor{})
	svc.registerMessageHandlers()

	if svc.GetConsumer().GetStateManager("shein.tasks") != nil {
		t.Fatal("did not expect shared shein queue handler to be registered")
	}

	for bucket := 0; bucket < sheinBucketQueueCount; bucket++ {
		queueName := fmt.Sprintf("shein.tasks.bucket.%d", bucket)
		if svc.GetConsumer().GetStateManager(queueName) != nil {
			t.Fatalf("did not expect shein bucket handler %s to be registered", queueName)
		}
	}
}

func TestRabbitMQServiceLoadsDynamicStoreAssignmentsBeforeRegisteringHandlers(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:           config.NodeRoleTask,
			UseStoreQueues: true,
			NodeID:         "shein-listing-store-c",
		},
	}, logrus.New())

	svc.SetStoreAssignmentProvider(stubStoreAssignmentProvider{stores: []int64{431, 870}})
	svc.processorRegistry.RegisterProcessor("shein", noopProcessor{})

	svc.syncInitialStoreAssignments(context.Background())
	svc.registerMessageHandlers()

	if svc.GetConsumer().GetStateManager("shein.tasks") != nil {
		t.Fatal("did not expect shared shein queue handler to be registered after initial assignment sync")
	}

	for bucket := 0; bucket < sheinBucketQueueCount; bucket++ {
		queueName := fmt.Sprintf("shein.tasks.bucket.%d", bucket)
		if svc.GetConsumer().GetStateManager(queueName) != nil {
			t.Fatalf("did not expect shared shein bucket handler %s to be registered after initial assignment sync", queueName)
		}
	}

	for _, storeID := range []int64{431, 870} {
		queueName := fmt.Sprintf("shein.tasks.store.%d", storeID)
		if svc.GetConsumer().GetStateManager(queueName) == nil {
			t.Fatalf("expected dynamic store queue handler %s to be registered", queueName)
		}
	}
}

func TestRabbitMQServiceProviderForcesStoreOnlyModeWithoutConfigFlag(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:   config.NodeRoleTask,
			NodeID: "shein-listing-store-d",
		},
	}, logrus.New())

	svc.SetStoreAssignmentProvider(stubStoreAssignmentProvider{stores: []int64{181}})
	svc.processorRegistry.RegisterProcessor("shein", noopProcessor{})

	svc.syncInitialStoreAssignments(context.Background())
	svc.registerMessageHandlers()

	if svc.GetConsumer().GetStateManager("shein.tasks") != nil {
		t.Fatal("did not expect shared shein queue handler to be registered when provider is configured")
	}

	if svc.GetConsumer().GetStateManager("shein.tasks.bucket.0") != nil {
		t.Fatal("did not expect shared shein bucket handlers to be registered when provider is configured")
	}

	if svc.GetConsumer().GetStateManager("shein.tasks.store.181") == nil {
		t.Fatal("expected store queue handler to be registered when provider is configured")
	}
}

func TestHTTPServerHealthWhenRabbitMQDisconnected(t *testing.T) {
	logger := logrus.New()
	cfg := &config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:            config.NodeRoleTask,
			HealthCheckPort: 8081,
			MetricsPort:     8082,
		},
	}
	svc := NewRabbitMQService(cfg, logger)
	loadMonitor := rabbitmq.NewLoadMonitor(config.LoadMonitorConfig{}, logger)
	server := NewHTTPServerManager(cfg, loadMonitor, svc, nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	server.handleHealth(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected /health to return 200 when RabbitMQ is disconnected, got %d", recorder.Code)
	}
}

func TestHTTPServerReadinessWhenRabbitMQDisconnected(t *testing.T) {
	logger := logrus.New()
	cfg := &config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:            config.NodeRoleTask,
			HealthCheckPort: 8081,
			MetricsPort:     8082,
		},
	}
	svc := NewRabbitMQService(cfg, logger)
	loadMonitor := rabbitmq.NewLoadMonitor(config.LoadMonitorConfig{}, logger)
	server := NewHTTPServerManager(cfg, loadMonitor, svc, nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	recorder := httptest.NewRecorder()

	server.handleReady(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected /ready to return 503 when RabbitMQ is disconnected, got %d", recorder.Code)
	}
}

func TestRabbitMQServiceReportsUnhealthyRequiredConsumers(t *testing.T) {
	logger := logrus.New()
	cfg := &config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:            config.NodeRoleTask,
			HealthCheckPort: 8081,
			MetricsPort:     8082,
		},
	}
	svc := NewRabbitMQService(cfg, logger)

	svc.GetConsumer().RegisterHandler("shein.tasks", noopRabbitHandler{})
	svc.GetConsumer().RegisterHandler("amazon.tasks", noopRabbitHandler{})

	if svc.HasHealthyRequiredConsumers() {
		t.Fatal("expected registered queues without running consumers to be unhealthy")
	}

	svc.GetConsumer().GetStateManager("shein.tasks").SetState(rabbitmq.ConsumerStateRunning, "shein.tasks")
	svc.GetConsumer().GetStateManager("amazon.tasks").SetError(errors.New("worker stopped"), "amazon.tasks")

	unhealthy := svc.GetUnhealthyRequiredQueues()
	if len(unhealthy) != 1 || unhealthy[0] != "amazon.tasks" {
		t.Fatalf("unexpected unhealthy queues: %#v", unhealthy)
	}
}

func TestStartupRetryDelayCapsAtThirtySeconds(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		base     time.Duration
		expected time.Duration
	}{
		{name: "first attempt uses base delay", attempt: 0, base: 5 * time.Second, expected: 5 * time.Second},
		{name: "delay grows exponentially", attempt: 2, base: 5 * time.Second, expected: 20 * time.Second},
		{name: "delay is capped", attempt: 5, base: 5 * time.Second, expected: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := startupRetryDelay(tt.attempt, tt.base)
			if got != tt.expected {
				t.Fatalf("expected delay %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestDecideConsumerAction(t *testing.T) {
	tests := []struct {
		name             string
		started          bool
		connected        bool
		consumerActive   bool
		consumersHealthy bool
		expected         consumerReconcileAction
	}{
		{
			name:             "not started does nothing",
			started:          false,
			connected:        true,
			consumerActive:   true,
			consumersHealthy: true,
			expected:         consumerActionNone,
		},
		{
			name:             "disconnect pauses active consumers",
			started:          true,
			connected:        false,
			consumerActive:   true,
			consumersHealthy: false,
			expected:         consumerActionPause,
		},
		{
			name:             "connected but inactive resumes consumers",
			started:          true,
			connected:        true,
			consumerActive:   false,
			consumersHealthy: false,
			expected:         consumerActionResume,
		},
		{
			name:             "connected active unhealthy restarts consumers",
			started:          true,
			connected:        true,
			consumerActive:   true,
			consumersHealthy: false,
			expected:         consumerActionRestart,
		},
		{
			name:             "connected active healthy does nothing",
			started:          true,
			connected:        true,
			consumerActive:   true,
			consumersHealthy: true,
			expected:         consumerActionNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideConsumerAction(tt.started, tt.connected, tt.consumerActive, tt.consumersHealthy)
			if got != tt.expected {
				t.Fatalf("expected action %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestNormalizeOwnedBuckets(t *testing.T) {
	got := normalizeOwnedBuckets([]int{7, 2, 2, -1, 8, 0})
	expected := []int{0, 2, 7}

	if len(got) != len(expected) {
		t.Fatalf("expected %d buckets, got %d", len(expected), len(got))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("expected bucket %d at index %d, got %d", expected[i], i, got[i])
		}
	}
}

func TestRabbitMQServicePreloadOwnedStoreConfigsRequestsEachStore(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())

	storeAPI := &countingStoreAPI{
		stubStoreAPI: stubStoreAPI{
			store: &managementapi.StoreRespDTO{ID: 1},
		},
	}
	svc.storeAPI = storeAPI

	svc.preloadOwnedStoreConfigs([]int64{11, 22, 33})

	expected := []int64{11, 22, 33}
	if len(storeAPI.calls) != len(expected) {
		t.Fatalf("expected %d preload calls, got %d", len(expected), len(storeAPI.calls))
	}
	for i := range expected {
		if storeAPI.calls[i] != expected[i] {
			t.Fatalf("expected preload call for store %d at index %d, got %d", expected[i], i, storeAPI.calls[i])
		}
	}
}

func TestStoreAssignmentSyncCoordinatorShouldRunOnlyWithDynamicAssignments(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:   config.NodeRoleTask,
			NodeID: "node-a",
		},
	}, logrus.New())
	coordinator := newStoreAssignmentSyncCoordinator(svc)

	if coordinator.shouldRun(storeAssignmentSyncState{}) {
		t.Fatal("expected empty state to skip sync")
	}

	state := storeAssignmentSyncState{
		provider:       stubStoreAssignmentProvider{stores: []int64{1}},
		useStoreQueues: true,
		ctx:            context.Background(),
		nodeID:         "node-a",
	}
	if !coordinator.shouldRun(state) {
		t.Fatal("expected valid dynamic assignment state to run")
	}
}

func TestConsumerGuardCoordinatorSnapshotStateReflectsServiceHealth(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())
	svc.started = true
	svc.consumerActive = true
	svc.ctx = context.Background()
	svc.GetConsumer().RegisterHandler("shein.tasks", noopRabbitHandler{})

	state := newConsumerGuardCoordinator(svc).snapshotState()
	if !state.started || !state.consumerActive {
		t.Fatalf("state = %+v, want started and active", state)
	}
	if state.ctx == nil {
		t.Fatal("expected guard state to capture service context")
	}
	if state.connected {
		t.Fatal("expected disconnected service in test snapshot")
	}
	if state.consumersHealthy {
		t.Fatal("expected unhealthy consumers before queue workers are running")
	}
}

func TestQueueHandlerBuilderDetectsCrawlerPlatforms(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleHybrid,
		},
	}, logrus.New())

	builder := newQueueHandlerBuilder(svc)
	if !builder.isCrawlerPlatform("amazon.crawler") {
		t.Fatal("expected amazon.crawler to be detected as crawler platform")
	}
	if builder.isCrawlerPlatform("shein") {
		t.Fatal("did not expect shein to be detected as crawler platform")
	}
}

func TestRabbitMQServiceInitializeCrawlerQueuesSkipsEmptyRegions(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleCrawler,
		},
	}, logrus.New())

	if err := svc.initializeCrawlerQueues(); err != nil {
		t.Fatalf("initializeCrawlerQueues() error = %v", err)
	}
}

func TestApplyRabbitMQServiceDefaultsFillsMissingValues(t *testing.T) {
	cfg := &config.RabbitMQConfig{}

	applyRabbitMQServiceDefaults(cfg)

	if cfg.ReconnectInterval != 5*time.Second {
		t.Fatalf("ReconnectInterval = %v, want 5s", cfg.ReconnectInterval)
	}
	if cfg.MaxReconnectTries != 10 {
		t.Fatalf("MaxReconnectTries = %d, want 10", cfg.MaxReconnectTries)
	}
	if cfg.Consumer.PrefetchCount != 1 {
		t.Fatalf("PrefetchCount = %d, want 1", cfg.Consumer.PrefetchCount)
	}
	if cfg.Consumer.RetryDelay != 5*time.Second {
		t.Fatalf("RetryDelay = %v, want 5s", cfg.Consumer.RetryDelay)
	}
	if cfg.Consumer.MaxRetries != 3 {
		t.Fatalf("MaxRetries = %d, want 3", cfg.Consumer.MaxRetries)
	}
}

func TestSetStoreAssignmentProviderForcesStoreQueueMode(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())

	if svc.useStoreQueues {
		t.Fatal("expected store queue mode to be disabled before provider is set")
	}

	svc.SetStoreAssignmentProvider(stubStoreAssignmentProvider{stores: []int64{11}})

	if !svc.useStoreQueues {
		t.Fatal("expected store queue mode to be enabled after provider is set")
	}
}

func TestRabbitMQServiceRoutingStateSnapshotCopiesRoutingSlices(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:         config.NodeRoleTask,
			OwnedBuckets: []int{2, 4},
		},
	}, logrus.New())
	svc.ownedStores = []int64{7, 9}
	svc.useStoreQueues = true

	state := svc.routingStateSnapshot()
	if !state.useStoreQueues {
		t.Fatal("expected routing state to preserve useStoreQueues")
	}
	if len(state.ownedStores) != 2 || state.ownedStores[0] != 7 || state.ownedStores[1] != 9 {
		t.Fatalf("owned stores = %v, want [7 9]", state.ownedStores)
	}
	if len(state.ownedBuckets) != 2 || state.ownedBuckets[0] != 2 || state.ownedBuckets[1] != 4 {
		t.Fatalf("owned buckets = %v, want [2 4]", state.ownedBuckets)
	}

	svc.ownedStores[0] = 99
	svc.ownedBuckets[0] = 8
	if state.ownedStores[0] != 7 || state.ownedBuckets[0] != 2 {
		t.Fatal("expected routing state snapshot to copy routing slices")
	}
}

func TestRabbitMQServiceStopWaitsForBackgroundWorkers(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())

	serviceCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopped := make(chan struct{})
	svc.ctx = serviceCtx
	svc.cancel = cancel
	svc.started = true
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		<-serviceCtx.Done()
		close(stopped)
	}()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := svc.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("expected Stop to wait for background worker shutdown")
	}
}

func TestRabbitMQServiceConsumerLifecycleStateSnapshotReflectsFlagsAndContext(t *testing.T) {
	serviceCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())
	svc.started = true
	svc.consumerActive = true
	svc.ctx = serviceCtx

	state := svc.consumerLifecycleStateSnapshot()
	if !state.started || !state.consumerActive {
		t.Fatalf("state = %+v, want started and active", state)
	}
	if state.ctx != serviceCtx {
		t.Fatal("expected lifecycle snapshot to preserve service context")
	}
}

func TestRabbitMQServicePrepareStartStateAssignsServiceContext(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())

	state, err := svc.prepareStartState(parentCtx)
	if err != nil {
		t.Fatalf("prepareStartState() error = %v", err)
	}
	if state.ctx == nil || state.cancel == nil {
		t.Fatal("expected prepared start state to include ctx and cancel")
	}
	if svc.ctx != state.ctx || fmt.Sprintf("%p", svc.cancel) != fmt.Sprintf("%p", state.cancel) {
		t.Fatal("expected prepareStartState to persist service start context")
	}
	if svc.started {
		t.Fatal("expected prepareStartState to not mark service started")
	}
}

func TestRabbitMQServiceStopStateSnapshotCapturesStopDependencies(t *testing.T) {
	serviceCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := stubStoreAssignmentProvider{stores: []int64{11}}
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())
	svc.started = true
	svc.consumerActive = true
	svc.ctx = serviceCtx
	svc.cancel = cancel
	svc.storeAssignmentProvider = provider

	state := svc.stopStateSnapshot()
	if !state.started {
		t.Fatal("expected stop state to preserve started flag")
	}
	if state.consumer != svc.consumer {
		t.Fatal("expected stop state to preserve consumer")
	}
	if state.cancel == nil {
		t.Fatal("expected stop state to preserve cancel func")
	}
	providerState, ok := state.provider.(stubStoreAssignmentProvider)
	if !ok {
		t.Fatalf("provider type = %T, want stubStoreAssignmentProvider", state.provider)
	}
	if len(providerState.stores) != 1 || providerState.stores[0] != 11 {
		t.Fatalf("provider stores = %v, want [11]", providerState.stores)
	}
}

func TestRabbitMQServiceStatsStateSnapshotCopiesRoutingState(t *testing.T) {
	svc := NewRabbitMQService(&config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role: config.NodeRoleTask,
		},
	}, logrus.New())
	svc.started = true
	svc.ownedStores = []int64{11, 22}
	svc.ownedBuckets = []int{1, 3}
	svc.useStoreQueues = true

	state := svc.statsStateSnapshot()
	if !state.started || !state.useStoreQueues {
		t.Fatalf("state = %+v, want started and store queues enabled", state)
	}
	if len(state.ownedStores) != 2 || state.ownedStores[0] != 11 || state.ownedStores[1] != 22 {
		t.Fatalf("owned stores = %v, want [11 22]", state.ownedStores)
	}
	if len(state.ownedBuckets) != 2 || state.ownedBuckets[0] != 1 || state.ownedBuckets[1] != 3 {
		t.Fatalf("owned buckets = %v, want [1 3]", state.ownedBuckets)
	}

	svc.ownedStores[0] = 99
	svc.ownedBuckets[0] = 9
	if state.ownedStores[0] != 11 || state.ownedBuckets[0] != 1 {
		t.Fatal("expected stats snapshot to copy routing slices")
	}
}
