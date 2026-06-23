# SHEIN Listing Ownership Controller Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split SHEIN listing ownership coordination from shard workers so only a coordinator writes Redis store ownership, workers only read ownership, and dedicated stores like 976 cannot be assigned to shared shards.

**Architecture:** Add an explicit `rabbitmq.autoShard.role` with `coordinator`, `worker`, and `disabled` effective behavior. Update SHEIN runtime wiring so the coordinator starts `AutoShardCoordinator` without dynamic consumer assignment, while shared shard workers use Redis assignment without writing ownership. Add Kubernetes manifests for a single ownership controller Deployment and make the shard StatefulSet role `worker`.

**Tech Stack:** Go, Viper config, Kubernetes/Kustomize, Redis-backed store ownership, RabbitMQ worker runtime.

---

## Existing State To Preserve

The working tree already contains production hotfix changes:

- `internal/app/consumer/auto_shard_coordinator.go` skips `DedicatedQueueEnabled=true`.
- `internal/app/consumer/auto_shard_coordinator_test.go` tests that skip.
- `internal/infra/clients/management/api/store.go` includes `DedicatedQueueEnabled`.
- `deployments/kubernetes/shein-listing/overlays/prod-single-store-976/` exists.
- `deployments/kubernetes/shein-listing/stores.single-pod.json` includes store 976.
- `deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/statefulset.yaml` has the latest image from the hotfix rollout.

Do not revert these changes. Build on them.

## File Structure

- Modify `internal/core/config/type_rabbitmq.go`: define role constants and effective role helpers.
- Modify `internal/core/config/loader_rabbitmq.go`: load `rabbitmq.autoShard.role`.
- Modify `internal/core/config/config.go`: bind `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE`.
- Modify `internal/core/config/validator_rabbitmq.go`: validate auto-shard role and candidate node requirements.
- Modify config tests under `internal/core/config`: cover env loading, defaults, validation.
- Modify `internal/platforms/shein/module.go`: gate dynamic assignment and coordinator startup by effective role.
- Modify or add SHEIN platform tests under `internal/platforms/shein`: cover coordinator/worker/disabled runtime wiring.
- Modify `internal/app/consumer/auto_shard_coordinator_test.go`: keep dedicated skip test and add role-adjacent tests only if they naturally belong there.
- Modify `deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/statefulset.yaml`: set shard role to worker.
- Add `deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/ownership-controller.yaml`: coordinator Deployment.
- Modify `deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/kustomization.yaml`: include the controller.
- Modify `deployments/kubernetes/shein-listing/templates/single-store-deployment.yaml.tpl`: set dedicated pods to role disabled.
- Modify `deployments/kubernetes/shein-listing/overlays/prod-single-store-976/deployment.yaml`: set role disabled.

---

### Task 1: Auto-Shard Role Config

**Files:**
- Modify: `internal/core/config/type_rabbitmq.go`
- Modify: `internal/core/config/loader_rabbitmq.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/validator_rabbitmq.go`
- Test: `internal/core/config/config_env_test.go`
- Test: `internal/core/config/validator_rabbitmq_test.go` or the existing validator test file if named differently

- [ ] **Step 1: Write failing tests for role loading and defaults**

Add tests that prove:

```go
func TestRabbitMQAutoShardRoleEnvironmentVariable(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED", "true")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE", "worker")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_CANDIDATE_NODES", "node-a,node-b")

	v := NewViper()
	cfg := BuildRabbitMQConfig(v)

	assert.Equal(t, "worker", cfg.AutoShard.Role)
	assert.Equal(t, AutoShardRoleWorker, cfg.AutoShard.EffectiveRole())
	assert.True(t, cfg.AutoShard.IsWorker())
	assert.False(t, cfg.AutoShard.IsCoordinator())
}

func TestAutoShardEffectiveRoleDefaults(t *testing.T) {
	assert.Equal(t, AutoShardRoleDisabled, AutoShardConfig{}.EffectiveRole())
	assert.Equal(t, AutoShardRoleCoordinator, AutoShardConfig{Enabled: true}.EffectiveRole())
	assert.Equal(t, AutoShardRoleDisabled, AutoShardConfig{Enabled: true, Role: "disabled"}.EffectiveRole())
}
```

Add validation tests that prove:

```go
func TestValidateRabbitMQRejectsInvalidAutoShardRole(t *testing.T) {
	cfg := &RabbitMQConfig{
		Enabled: true,
		Node: NodeConfig{HealthCheckPort: 8081, MetricsPort: 8082},
		AutoShard: AutoShardConfig{
			Enabled: true,
			Role: "writer",
			Interval: time.Second,
			LockTTL: time.Second,
			CandidateNodes: []string{"node-a"},
		},
	}

	errs := validateRabbitMQConfig(cfg)
	require.NotEmpty(t, errs)
	require.Contains(t, validationFields(errs), "rabbitmq.autoShard.role")
}

func TestValidateRabbitMQAllowsWorkerWithoutCandidateNodes(t *testing.T) {
	cfg := &RabbitMQConfig{
		Enabled: true,
		Node: NodeConfig{HealthCheckPort: 8081, MetricsPort: 8082},
		AutoShard: AutoShardConfig{
			Enabled: true,
			Role: AutoShardRoleWorker,
			Interval: time.Second,
			LockTTL: time.Second,
		},
	}

	errs := validateRabbitMQConfig(cfg)
	require.NotContains(t, validationFields(errs), "rabbitmq.autoShard.candidateNodes")
}
```

Use the repository's real validator function and helper naming. If there is no `validationFields`, add a tiny test helper in the test file.

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/core/config -run "AutoShardRole|RabbitMQAutoShardRole|ValidateRabbitMQ.*AutoShard" -count=1
```

Expected: FAIL because `AutoShardConfig.Role`, role constants, helpers, env binding, or validation do not exist yet.

- [ ] **Step 3: Implement config role support**

In `internal/core/config/type_rabbitmq.go`, add:

```go
const (
	AutoShardRoleCoordinator = "coordinator"
	AutoShardRoleWorker      = "worker"
	AutoShardRoleDisabled    = "disabled"
)
```

Add `Role string` to `AutoShardConfig`.

Add helpers:

```go
func (c AutoShardConfig) EffectiveRole() string {
	if !c.Enabled {
		return AutoShardRoleDisabled
	}
	switch strings.ToLower(strings.TrimSpace(c.Role)) {
	case "", AutoShardRoleCoordinator:
		return AutoShardRoleCoordinator
	case AutoShardRoleWorker:
		return AutoShardRoleWorker
	case AutoShardRoleDisabled:
		return AutoShardRoleDisabled
	default:
		return strings.ToLower(strings.TrimSpace(c.Role))
	}
}

func (c AutoShardConfig) HasValidRole() bool {
	switch c.EffectiveRole() {
	case AutoShardRoleCoordinator, AutoShardRoleWorker, AutoShardRoleDisabled:
		return true
	default:
		return false
	}
}

func (c AutoShardConfig) IsCoordinator() bool {
	return c.EffectiveRole() == AutoShardRoleCoordinator
}

func (c AutoShardConfig) IsWorker() bool {
	return c.EffectiveRole() == AutoShardRoleWorker
}
```

Import `strings` in that file if needed.

In `loader_rabbitmq.go`, populate:

```go
Role: v.GetString("rabbitmq.autoShard.role"),
```

In `config.go`, add an env binding for:

```text
rabbitmq.autoShard.role -> TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE
```

In `validator_rabbitmq.go`, validate invalid roles and only require candidate nodes when `AutoShard.Enabled && AutoShard.IsCoordinator()`.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```powershell
gofmt -w internal/core/config/type_rabbitmq.go internal/core/config/loader_rabbitmq.go internal/core/config/config.go internal/core/config/validator_rabbitmq.go internal/core/config/*test.go
go test ./internal/core/config -run "AutoShardRole|RabbitMQAutoShardRole|ValidateRabbitMQ.*AutoShard" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit config role support**

```powershell
git add internal/core/config
git commit -m "feat: add auto shard runtime roles"
```

---

### Task 2: SHEIN Runtime Role Wiring

**Files:**
- Modify: `internal/platforms/shein/module.go`
- Test: `internal/platforms/shein/module_test.go` or existing SHEIN module test file

- [ ] **Step 1: Write failing tests for coordinator and worker wiring**

Add tests with a fake `ServiceManager` if existing tests already have one; otherwise test extracted pure helpers from `module.go`.

Prefer extracting these pure helpers:

```go
func shouldEnableDynamicStoreAssignment(cfg *config.Config) bool
func shouldConfigureAutoShard(cfg *config.Config) bool
```

Tests:

```go
func TestShouldEnableDynamicStoreAssignmentForWorkerRole(t *testing.T) {
	cfg := &config.Config{
		Redis: &config.RedisConfig{},
		RabbitMQ: &config.RabbitMQConfig{
			Node: config.NodeConfig{UseStoreQueues: true},
			AutoShard: config.AutoShardConfig{Enabled: true, Role: config.AutoShardRoleWorker},
		},
	}

	require.True(t, shouldEnableDynamicStoreAssignment(cfg))
	require.False(t, shouldConfigureAutoShard(cfg))
}

func TestShouldConfigureAutoShardOnlyForCoordinatorRole(t *testing.T) {
	cfg := &config.Config{
		Redis: &config.RedisConfig{},
		RabbitMQ: &config.RabbitMQConfig{
			Node: config.NodeConfig{UseStoreQueues: false},
			AutoShard: config.AutoShardConfig{Enabled: true, Role: config.AutoShardRoleCoordinator},
		},
	}

	require.False(t, shouldEnableDynamicStoreAssignment(cfg))
	require.True(t, shouldConfigureAutoShard(cfg))
}

func TestDedicatedStaticStoreDoesNotUseDynamicAssignment(t *testing.T) {
	cfg := &config.Config{
		Redis: &config.RedisConfig{},
		RabbitMQ: &config.RabbitMQConfig{
			Node: config.NodeConfig{UseStoreQueues: true, OwnedStores: []int64{976}},
			AutoShard: config.AutoShardConfig{Enabled: false, Role: config.AutoShardRoleDisabled},
		},
	}

	require.False(t, shouldEnableDynamicStoreAssignment(cfg))
	require.False(t, shouldConfigureAutoShard(cfg))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/platforms/shein -run "ShouldEnableDynamicStoreAssignment|ShouldConfigureAutoShard|DedicatedStaticStore" -count=1
```

Expected: FAIL because helper functions do not exist or current behavior starts coordinator on workers.

- [ ] **Step 3: Implement role-gated wiring**

In `internal/platforms/shein/module.go`, replace direct conditions with helpers:

```go
if shouldEnableDynamicStoreAssignment(cfg) {
	if err := consumer.EnableDynamicStoreAssignment(cfg, rt.Logger, rt.ServiceManager); err != nil {
		return err
	}
} else if cfg.RabbitMQ.Node.UseStoreQueues && len(cfg.RabbitMQ.Node.OwnedStores) > 0 {
	rt.Logger.Infof("static store assignment enabled: nodeID=%s, ownedStores=%v", cfg.RabbitMQ.Node.NodeID, cfg.RabbitMQ.Node.OwnedStores)
}

if shouldConfigureAutoShard(cfg) {
	if err := configureAutoShard(rt); err != nil {
		return err
	}
}
```

Add:

```go
func shouldEnableDynamicStoreAssignment(cfg *config.Config) bool {
	return cfg != nil &&
		cfg.RabbitMQ != nil &&
		cfg.RabbitMQ.Node.UseStoreQueues &&
		len(cfg.RabbitMQ.Node.OwnedStores) == 0 &&
		cfg.Redis != nil &&
		!cfg.RabbitMQ.AutoShard.IsCoordinator()
}

func shouldConfigureAutoShard(cfg *config.Config) bool {
	return cfg != nil &&
		cfg.RabbitMQ != nil &&
		cfg.RabbitMQ.AutoShard.IsCoordinator()
}
```

This keeps dedicated static pods out of dynamic assignment and keeps workers from starting the coordinator.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```powershell
gofmt -w internal/platforms/shein/module.go internal/platforms/shein/*test.go
go test ./internal/platforms/shein -run "ShouldEnableDynamicStoreAssignment|ShouldConfigureAutoShard|DedicatedStaticStore" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit runtime wiring**

```powershell
git add internal/platforms/shein
git commit -m "feat: separate shein shard coordinator role"
```

---

### Task 3: Dedicated Store Field Propagation Tests

**Files:**
- Modify: `internal/infra/clients/management/api/store.go`
- Modify/Test: `internal/infra/clients/management/local_noauth_clients_test.go`
- Modify/Test: local provider files only if tests show field loss
- Test: `internal/app/consumer/auto_shard_coordinator_test.go`

- [ ] **Step 1: Write or extend failing field propagation tests**

Add a test proving `DedicatedQueueEnabled` survives management DTO JSON:

```go
func TestStoreRespDTOIncludesDedicatedQueueEnabled(t *testing.T) {
	enabled := true
	raw, err := json.Marshal(api.StoreRespDTO{ID: 976, DedicatedQueueEnabled: &enabled})
	require.NoError(t, err)
	require.Contains(t, string(raw), "dedicatedQueueEnabled")

	var decoded api.StoreRespDTO
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NotNil(t, decoded.DedicatedQueueEnabled)
	require.True(t, *decoded.DedicatedQueueEnabled)
}
```

If local management provider has store round-trip tests, add a row with `dedicated_queue_enabled=true` and assert returned DTO has `DedicatedQueueEnabled=true`.

- [ ] **Step 2: Run tests**

Run:

```powershell
go test ./internal/infra/clients/management/... -run "DedicatedQueueEnabled|StoreRespDTO" -count=1
go test ./internal/app/consumer -run TestListEligibleStoresSkipsDedicatedQueueStores -count=1
```

Expected: PASS if the hotfix already covers DTO and coordinator. If local provider fails, implement the missing mapping.

- [ ] **Step 3: Implement missing local provider mapping if needed**

If tests fail, update local provider model-to-DTO conversion to copy:

```go
DedicatedQueueEnabled: row.DedicatedQueueEnabled,
```

Use the exact model field name in the local provider.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```powershell
gofmt -w internal/infra/clients/management internal/app/consumer
go test ./internal/infra/clients/management/... -run "DedicatedQueueEnabled|StoreRespDTO" -count=1
go test ./internal/app/consumer -run TestListEligibleStoresSkipsDedicatedQueueStores -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit field propagation**

```powershell
git add internal/infra/clients/management internal/app/consumer
git commit -m "fix: propagate dedicated queue store flag"
```

---

### Task 4: Kubernetes Ownership Controller

**Files:**
- Modify: `deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/statefulset.yaml`
- Create: `deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/ownership-controller.yaml`
- Modify: `deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/kustomization.yaml`
- Modify: `deployments/kubernetes/shein-listing/templates/single-store-deployment.yaml.tpl`
- Modify: `deployments/kubernetes/shein-listing/overlays/prod-single-store-976/deployment.yaml`

- [ ] **Step 1: Update shard StatefulSet role**

In `statefulset.yaml`, add after `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED`:

```yaml
- name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE
  value: worker
```

Keep candidate nodes and `USE_STORE_QUEUES=true`; workers still need dynamic Redis assignment.

- [ ] **Step 2: Add ownership controller Deployment**

Create `ownership-controller.yaml` based on the shard container but with one replica and coordinator role:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shein-listing-ownership-controller
  namespace: task-processor
  labels:
    app: shein-listing-ownership-controller
    shein-listing-role: ownership-controller
spec:
  replicas: 1
  revisionHistoryLimit: 3
  selector:
    matchLabels:
      app: shein-listing-ownership-controller
  template:
    metadata:
      labels:
        app: shein-listing-ownership-controller
        shein-listing-role: ownership-controller
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/agent
                    operator: In
                    values: ["true"]
                  - key: task-processor/crawler-tier
                    operator: In
                    values: ["heavy"]
      containers:
        - name: shein-listing
          image: xuwei190/task-processor-shein-listing:e67b5cab-dedicated-queue-fix-20260622-2258
          imagePullPolicy: IfNotPresent
          command: ["/app/listing-service"]
          args:
            - -config
            - /app/config/config-shein-listing.yaml
            - -log-level
            - info
          ports:
            - name: http
              containerPort: 8081
            - name: metrics
              containerPort: 8082
          env:
            - name: TZ
              value: Asia/Shanghai
            - name: TASK_PROCESSOR_ENV
              value: prod
            - name: TASK_PROCESSOR_RABBITMQ_NODE_NODE_ID
              value: shein-listing-ownership-controller
            - name: TASK_PROCESSOR_RABBITMQ_NODE_ROLE
              value: task
            - name: TASK_PROCESSOR_RABBITMQ_NODE_USE_STORE_QUEUES
              value: "false"
            - name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED
              value: "true"
            - name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE
              value: coordinator
            - name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_PLATFORM
              value: shein
            - name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_INTERVAL
              value: "30"
            - name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_PAGE_SIZE
              value: "200"
            - name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_LOCK_KEY
              value: listing:queue:auto-shard:lock
            - name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_LOCK_TTL
              value: "25"
            - name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_CANDIDATE_NODES
              value: shein-listing-shard-0,shein-listing-shard-1,shein-listing-shard-2,shein-listing-shard-3,shein-listing-shard-4,shein-listing-shard-5,shein-listing-shard-6,shein-listing-shard-7,shein-listing-shard-8,shein-listing-shard-9,shein-listing-shard-10,shein-listing-shard-11,shein-listing-shard-12,shein-listing-shard-13,shein-listing-shard-14,shein-listing-shard-15,shein-listing-shard-16,shein-listing-shard-17,shein-listing-shard-18,shein-listing-shard-19,shein-listing-shard-20,shein-listing-shard-21,shein-listing-shard-22,shein-listing-shard-23,shein-listing-shard-24,shein-listing-shard-25,shein-listing-shard-26,shein-listing-shard-27,shein-listing-shard-28,shein-listing-shard-29,shein-listing-shard-30,shein-listing-shard-31
            - name: TASK_PROCESSOR_REDIS_DB
              value: "9"
            - name: TASK_PROCESSOR_RABBITMQ_NODE_HEALTH_CHECK_PORT
              value: "8081"
            - name: TASK_PROCESSOR_RABBITMQ_NODE_METRICS_PORT
              value: "8082"
          envFrom:
            - secretRef:
                name: shein-listing-secret
          resources:
            requests:
              cpu: "500m"
              memory: "1000Mi"
            limits:
              cpu: "1"
              memory: "2Gi"
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 10
            periodSeconds: 5
            timeoutSeconds: 5
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 120
            periodSeconds: 30
            timeoutSeconds: 10
            failureThreshold: 6
          startupProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 10
            failureThreshold: 90
          volumeMounts:
            - name: app-config
              mountPath: /app/config/config-shein-listing.yaml
              subPath: config-shein-listing.yaml
            - name: app-tmp
              mountPath: /app/tmp
            - name: app-logs
              mountPath: /app/logs
      volumes:
        - name: app-config
          configMap:
            name: shein-listing-config-heavy
        - name: app-tmp
          emptyDir: {}
        - name: app-logs
          emptyDir: {}
```

- [ ] **Step 3: Add controller to kustomization**

In `kustomization.yaml`, add:

```yaml
  - ownership-controller.yaml
```

- [ ] **Step 4: Mark dedicated pod templates disabled**

Add this env var to both the template and 976 manifest:

```yaml
- name: TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE
  value: disabled
```

Place it next to `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED`.

- [ ] **Step 5: Validate manifests**

Run:

```powershell
kubectl kustomize deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset > $env:TEMP\shein-ownership-rendered.yaml
Select-String -Path $env:TEMP\shein-ownership-rendered.yaml -Pattern 'shein-listing-ownership-controller|TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE|value: worker|value: coordinator'
```

Expected: rendered YAML includes controller Deployment and worker/coordinator role env vars.

- [ ] **Step 6: Commit manifests**

```powershell
git add deployments/kubernetes/shein-listing
git commit -m "deploy: add shein listing ownership controller"
```

---

### Task 5: Full Verification

**Files:**
- No code changes unless verification reveals a defect.

- [ ] **Step 1: Run focused package tests**

```powershell
go test ./internal/core/config -count=1
go test ./internal/platforms/shein -count=1
go test ./internal/app/consumer -count=1
go test ./internal/infra/clients/management/... -count=1
```

Expected: PASS.

- [ ] **Step 2: Run build for SHEIN listing command**

```powershell
go test ./cmd/shein-listing -count=1
go build ./cmd/shein-listing
```

Expected: PASS/build succeeds.

- [ ] **Step 3: Verify working tree scope**

```powershell
git status --short
git diff --stat
```

Expected: only files related to this ownership-controller phase are changed, plus the previously accepted 976 hotfix files if not already committed.

- [ ] **Step 4: Document deployment order**

Add final rollout notes to the PR or final response:

```text
1. Build and push a new shein-listing image.
2. Apply prod-auto-shard-statefulset overlay so ownership controller exists.
3. Roll shein-listing-shard with autoShard.role=worker.
4. Roll dedicated store 976 with autoShard.role=disabled.
5. Delete temporary Redis lock listing:queue:auto-shard:lock.
6. Verify Redis/RabbitMQ ownership for store 976.
```

- [ ] **Step 5: Commit verification doc updates if any**

If deployment notes are added to a README:

```powershell
git add deployments/kubernetes/shein-listing/README.md
git commit -m "docs: document shein ownership rollout"
```

Skip this commit if no docs are changed.

---

## Self-Review

Spec coverage:

- Role contract: Task 1.
- Runtime behavior: Task 2.
- Dedicated flag propagation: Task 3.
- Kubernetes controller and worker split: Task 4.
- Verification and rollout order: Task 5.

Placeholder scan:

- No TBD/TODO placeholders.
- Each task has concrete files, commands, and expected outcomes.

Type consistency:

- Role constants use `AutoShardRoleCoordinator`, `AutoShardRoleWorker`, and `AutoShardRoleDisabled`.
- Config path uses `rabbitmq.autoShard.role`.
- Env var uses `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE`.
