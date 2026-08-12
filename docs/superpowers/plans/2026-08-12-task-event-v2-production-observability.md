# TaskEvent V2 Production Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Release TaskEvent V2-compatible SHEIN consumers through a one-pod canary and establish durable evidence for the 14-day legacy-event removal gate.

**Architecture:** Install the upstream kube-prometheus-stack in monitoring, then apply repository-owned PodMonitor, PrometheusRule, and Grafana resources. Extend the existing StatefulSet helper with a stop-after-one-pod canary mode; use it before rolling the ten shard workers and the separately labelled dedicated store.

**Tech Stack:** Kubernetes 1.36/K3s, Helm OCI chart kube-prometheus-stack 88.3.0, Prometheus Operator, Grafana, Kustomize, PowerShell/Pester, Go.

## Execution Status (2026-08-12)

- Tasks 1-4 are implemented and committed on the dedicated branch. Verified with the pinned Helm render, Kustomize render, Pester canary tests, focused Go tests, architecture tests, and the full Go baseline before changes.
- Task 5 is deliberately pending. It performs Helm, image-registry, and Kubernetes production writes and requires a fresh explicit production authorization after current-state preflight.
- The workstation provides Pester 3.4.0; its compatible invocation is `Invoke-Pester -Script <path> -Verbose`, rather than the newer `-Output Detailed` syntax.

## Global Constraints

- Use OCI chart oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack version 88.3.0, digest sha256:2b2f2c5c6f76ff661bdb34e6ae3410e01258f39632bffa52b94c0d42a1da0be6.
- Install in monitoring using local-path; use retention 21d, a 30Gi RWO PVC, and no retentionSize limit.
- Prometheus, Grafana, Alertmanager and kube-state-metrics normal memory requests must remain below 4Gi; remove the obsolete fixed Alertmanager affinity.
- Do not create public Grafana or Alertmanager ingress and never commit credentials.
- Update only statefulset/shein-listing-shard (10 replicas) and deployment/shein-listing-store-883 (one replica). Do not start or modify store-976.
- Do not alter Listing Control Plane, RabbitMQ claim/ack order, or remove the legacy decoder. No Helm install, image push, or cluster write without separate production authorization.

---

## File Structure

- Modify: deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml — retention, PVC and small-cluster resources.
- Create: deployments/kubernetes/monitoring/shein-listing/podmonitor-store-883.yaml — metrics discovery for the active dedicated store.
- Modify: deployments/kubernetes/monitoring/shein-listing/kustomization.yaml, prometheusrule.yaml and grafana-dashboard-configmap.yaml — discovery, V2 rules and dashboard panels.
- Modify: deployments/kubernetes/monitoring/README.md and docs/architecture/task-event-v2-migration.md — reproducible operator and evidence runbook.
- Modify: scripts/rollout-shein-shard-statefulset.ps1; create scripts/rollout-shein-shard-statefulset.Tests.ps1 — one-pod canary without changing normal rollout semantics.
- Modify: internal/infra/metrics/consumer_registry_test.go — lock the public low-cardinality metric contract.

## Task 1: Pin and render the monitoring stack

**Files:**
- Modify: deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml
- Modify: deployments/kubernetes/monitoring/README.md

**Interfaces:**
- Consumes: chart 88.3.0 and storage class local-path.
- Produces: values for a pinned atomic Helm install.

- [ ] **Step 1: Record the failing retention baseline**

Run:

~~~powershell
helm template monitoring oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack --version 88.3.0 --namespace monitoring -f deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml | Select-String -Pattern 'retention:\s+"7d"'
~~~

Expected: match found; the existing seven-day configuration cannot prove a full 14-day window.

- [ ] **Step 2: Implement the minimum safe values**

Set Prometheus retention to 21d, remove retentionSize, and request 30Gi on local-path. Keep the existing 30-second intervals. Add bounded resources, beginning with:

~~~yaml
prometheus:
  prometheusSpec:
    resources:
      requests: { cpu: 500m, memory: 2Gi }
      limits: { memory: 3Gi }
~~~

Give Grafana, Alertmanager and kube-state-metrics explicit bounded resources whose normal requests total less than 4Gi. Delete the Alertmanager affinity referring to vm-4-17-ubuntu.

- [ ] **Step 3: Render the exact chart**

~~~powershell
helm template monitoring oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack --version 88.3.0 --namespace monitoring -f deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml | Set-Content "$env:TEMP\task-event-v2-monitoring-rendered.yaml"
Select-String -Path "$env:TEMP\task-event-v2-monitoring-rendered.yaml" -Pattern 'retention:\s+"21d"|storage: 30Gi|kind: Prometheus|kind: CustomResourceDefinition'
~~~

Expected: CRDs, Prometheus, quoted 21d retention and 30Gi storage render; vm-4-17-ubuntu does not appear.

- [ ] **Step 4: Document immutable installation**

Update the monitoring README with the OCI chart version/digest, helm upgrade --install --atomic --timeout 10m, local-path preflight, and private Grafana access using port-forward only.

- [ ] **Step 5: Commit**

~~~powershell
git add deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml deployments/kubernetes/monitoring/README.md
git commit -m "feat(monitoring): retain task event migration evidence"
~~~

## Task 2: Observe and alert on every active consumer

**Files:**
- Create: deployments/kubernetes/monitoring/shein-listing/podmonitor-store-883.yaml
- Modify: deployments/kubernetes/monitoring/shein-listing/kustomization.yaml
- Modify: deployments/kubernetes/monitoring/shein-listing/prometheusrule.yaml
- Modify: deployments/kubernetes/monitoring/shein-listing/grafana-dashboard-configmap.yaml

**Interfaces:**
- Consumes: task_event_decoded_total{schema_version="legacy"}, shard label shein-listing-role=service, store label app=shein-listing-store-883.
- Produces: two discovered target classes and V2 migration alerts without high-cardinality labels.

- [ ] **Step 1: Reproduce the missing store target**

~~~powershell
kubectl kustomize deployments/kubernetes/monitoring/shein-listing | Select-String -Pattern 'app: shein-listing-store-883'
~~~

Expected: no match; the current monitor only finds role-labelled shards.

- [ ] **Step 2: Add the dedicated-store monitor**

Create the new PodMonitor with:

~~~yaml
spec:
  namespaceSelector:
    matchNames: [task-processor]
  selector:
    matchLabels:
      app: shein-listing-store-883
  podMetricsEndpoints:
    - port: metrics
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s
~~~

Copy the six existing namespace/pod/node relabelings exactly and register the file in kustomization.yaml.

- [ ] **Step 3: Add migration safety rules**

Append to the existing shein-listing group:

~~~yaml
- alert: TaskEventV2LegacyDecodeObserved
  expr: increase(task_event_decoded_total{schema_version="legacy",kubernetes_namespace="task-processor"}[15m]) > 0
  for: 0m
- alert: TaskEventV2MetricsScrapeMissing
  expr: (count(up{job=~"podMonitor/monitoring/(shein-listing|shein-listing-store-883)/.*"} == 1) or vector(0)) < 11
  for: 5m
~~~

Add a third rule that alerts after 15m if Prometheus PVC free space is below 20 percent. Annotations must say that a legacy event restarts the 14-day window and a scrape gap makes zero legacy count invalid. Do not add task, queue, tenant, or store labels.

- [ ] **Step 4: Add dashboard evidence panels and validate manifests**

Add panels for:

~~~promql
increase(task_event_decoded_total{schema_version="legacy",kubernetes_namespace="$namespace"}[14d])
sum by (pod) (up{namespace="$namespace",pod=~"$pod"})
~~~

Run:

~~~powershell
kubectl kustomize deployments/kubernetes/monitoring/shein-listing | Set-Content "$env:TEMP\shein-monitoring.yaml"
kubectl apply --dry-run=client -f "$env:TEMP\shein-monitoring.yaml"
~~~

Expected: all rendered YAML validates once Operator CRDs are present.

- [ ] **Step 5: Commit**

~~~powershell
git add deployments/kubernetes/monitoring/shein-listing
git commit -m "feat(monitoring): observe task event v2 migration"
~~~


## Task 3: Add a testable one-pod StatefulSet canary

**Files:**
- Modify: scripts/rollout-shein-shard-statefulset.ps1
- Create: scripts/rollout-shein-shard-statefulset.Tests.ps1

**Interfaces:**
- Consumes: -Image, -BatchSize, current StatefulSet replica count, and new switch -CanaryOnly.
- Produces: default full rollout unchanged; -CanaryOnly updates only highest ordinal and leaves partition at that ordinal for evidence review.

- [ ] **Step 1: Write the failing canary calculation test**

Refactor only the batch calculation into Get-RolloutBatches, returning objects with Partition and Ordinals. Add:

~~~powershell
Describe "Get-RolloutBatches" {
  It "returns only the highest ordinal for a 10-replica canary" {
    $batches = Get-RolloutBatches -Replicas 10 -BatchSize 4 -CanaryOnly
    $batches | Should -HaveCount 1
    $batches[0].Partition | Should -Be 9
    $batches[0].Ordinals | Should -Be @(9)
  }
}
~~~

Run: Invoke-Pester -Script ./scripts/rollout-shein-shard-statefulset.Tests.ps1 -Verbose  
Expected: FAIL because Get-RolloutBatches and -CanaryOnly do not exist.

- [ ] **Step 2: Implement the minimum pure batch contract**

Add [switch]$CanaryOnly. In the canary case return exactly:

~~~powershell
[pscustomobject]@{ Partition = $Replicas - 1; Ordinals = @($Replicas - 1) }
~~~

Replace the inline rollout loop with Get-RolloutBatches output. Do not change image update, revision detection, readiness polling, default BatchSize=4, or normal full-rollout output.

- [ ] **Step 3: Add normal and invalid-input regressions**

Add Pester cases asserting a 10-replica, batch-size 4 normal rollout uses ordinal batches 9..6, 5..2 and 1..0 with partitions 6, 2 and 0. Assert Replicas less than one and BatchSize less than one throw. This prevents canary support from silently changing existing production rollout behavior.

- [ ] **Step 4: Run focused validation**

~~~powershell
Invoke-Pester -Script ./scripts/rollout-shein-shard-statefulset.Tests.ps1 -Verbose
git diff --check
~~~

Expected: Pester passes and no whitespace error remains.

- [ ] **Step 5: Commit**

~~~powershell
git add scripts/rollout-shein-shard-statefulset.ps1 scripts/rollout-shein-shard-statefulset.Tests.ps1
git commit -m "feat(shein): add task event v2 canary rollout"
~~~

## Task 4: Lock the public metric contract and link the operations gate

**Files:**
- Modify: internal/infra/metrics/consumer_registry_test.go
- Modify: docs/architecture/task-event-v2-migration.md

**Interfaces:**
- Consumes: ConsumerRegistry snapshot field LegacyTaskEventDecodedCount.
- Produces: a regression test for the exact exported low-cardinality series and a single architecture-to-runbook link.

- [ ] **Step 1: Add the metric assertions**

In TestConsumerRegistryExportsSnapshotMetrics, add:

~~~go
require.Contains(t, output, `task_event_decoded_total{schema_version="legacy"} 2`)
require.NotContains(t, output, `task_id=`)
require.NotContains(t, output, `tenant_id=`)
require.NotContains(t, output, `store_id=`)
~~~

Run: go test ./internal/infra/metrics -run TestConsumerRegistryExportsSnapshotMetrics -count=1  
Expected: PASS. Temporarily changing schema_version in the assertion must fail, proving the test binds the external metric contract.

- [ ] **Step 2: Keep production code unchanged unless the test proves it broken**

The current registry already exposes the required counter. Do not add a duplicate counter. If the focused test exposes a discrepancy, fix only internal/infra/metrics/consumer_registry.go while keeping schema_version as the sole custom label.

- [ ] **Step 3: Update the migration architecture document**

Link the operational plan and add the exact removal-gate query:

~~~promql
increase(task_event_decoded_total{schema_version="legacy",kubernetes_namespace="task-processor"}[14d])
~~~

State that every active target must have healthy scrape coverage; legacy events, consumer rollback, and scrape gaps reset the observation window.

- [ ] **Step 4: Run integration checks**

~~~powershell
go test ./internal/infra/metrics ./internal/app/task ./internal/app/consumer -count=1
go test ./tests -count=1
git diff --check
~~~

Expected: tests pass, including architecture documentation guards.

- [ ] **Step 5: Commit**

~~~powershell
git add internal/infra/metrics/consumer_registry_test.go docs/architecture/task-event-v2-migration.md
git commit -m "test(task): lock legacy event migration metric"
~~~


## Task 5: Preflight, canary, rollback and release evidence

**Files:**
- Modify: deployments/kubernetes/monitoring/README.md
- Modify: docs/architecture/task-event-v2-migration.md

**Interfaces:**
- Consumes: pinned chart, rendered manifests, immutable SHEIN image digest, and -CanaryOnly.
- Produces: dated release evidence containing old image digests, target health, metric query results, and rollback commands.

- [ ] **Step 1: Run non-mutating preflight**

After Tasks 1-4 are committed and CI is green, run:

~~~powershell
git status --short
git rev-parse HEAD
go test ./... -count=1
kubectl -n task-processor get statefulset shein-listing-shard deployment/shein-listing-store-883 -o wide
kubectl get storageclass local-path
kubectl top node
helm template monitoring oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack --version 88.3.0 --namespace monitoring -f deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml | kubectl apply --dry-run=client -f -
kubectl kustomize deployments/kubernetes/monitoring/shein-listing | kubectl apply --dry-run=client -f -
~~~

Expected: clean tree, full Go suite green, both active workload identities present, storage present, approved memory headroom, and valid rendered manifests.

- [ ] **Step 2: Obtain explicit production authorization and install monitoring atomically**

Do not execute this step without a new explicit user authorization for production writes.

~~~powershell
helm upgrade --install monitoring oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack --version 88.3.0 --namespace monitoring --create-namespace --atomic --timeout 10m -f deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml
kubectl apply -k deployments/kubernetes/monitoring/shein-listing
kubectl -n monitoring get podmonitor,prometheusrule,prometheus,pod
~~~

Expected: two PodMonitors and V2 rules exist; Prometheus discovers the shard targets before any application rollout.

- [ ] **Step 3: Build a candidate and capture exact rollback state**

Build cmd/shein-listing from the reviewed commit with deployments/docker/Dockerfile.listing, push a non-floating version tag, inspect its registry digest, and record it. Before any workload mutation save:

~~~powershell
kubectl -n task-processor get statefulset/shein-listing-shard -o jsonpath='{.spec.template.spec.containers[?(@.name=="shein-listing")].image}{"\n"}'
kubectl -n task-processor get deployment/shein-listing-store-883 -o jsonpath='{.spec.template.spec.containers[?(@.name=="shein-listing")].image}{"\n"}'
~~~

Expected: candidate and rollback image references are digest-pinned or have a recorded registry digest.

- [ ] **Step 4: Run the one-pod canary and enforce its gate**

~~~powershell
.\scripts\rollout-shein-shard-statefulset.ps1 -Image '<candidate@sha256:...>' -CanaryOnly
~~~

Wait for two 30-second scrapes. Confirm the updated pod is ready, Prometheus reports up=1, its metrics include task_event_decoded_total, and RabbitMQ claim/ack, SHEIN failures and requeues are not worse than the pre-rollout baseline. If any condition fails, stop and run the same helper with the saved previous image before investigating.

- [ ] **Step 5: Complete only after canary approval**

Roll the remaining shards with the same immutable image, then update deployment/shein-listing-store-883, waiting for each rollout. Record:

~~~promql
count(up{namespace="task-processor",pod=~"shein-listing-shard-.*|shein-listing-store-883-.*"} == 1)
increase(task_event_decoded_total{schema_version="legacy",kubernetes_namespace="task-processor"}[14d])
~~~

Expected: 11 active targets healthy, no active old consumer image, and the 14-day query recorded as the start—not completion—of the compatibility-removal window.

- [ ] **Step 6: Preserve only safe release evidence**

Commit only non-sensitive runbook/evidence-template documentation. Never commit kubeconfigs, Grafana or Prometheus credentials, or pod logs containing tokens.

## Self-Review

- **Spec coverage:** Task 1 supplies the pinned persistent stack and removes stale affinity. Task 2 covers both active workload classes, alerts and Grafana. Task 3 supplies a one-pod stop gate. Task 4 locks the metric contract. Task 5 defines preflight, install, rollback and 14-day evidence.
- **No placeholders:** chart version/digest, workload names, metric name, retention, PVC size, selectors, rollout commands and acceptance queries are explicit. Production authorization is intentionally external rather than an omitted implementation step.
- **Type consistency:** all monitoring resources consume task_event_decoded_total with schema_version=legacy; later rollout instructions use the -CanaryOnly interface introduced in Task 3; no task invents a new Go production API.

## Execution Handoff

Plan complete and saved to docs/superpowers/plans/2026-08-12-task-event-v2-production-observability.md.

Because this plan culminates in production writes and the workspace is shared, Inline Execution is recommended: implement and review Tasks 1-4 here, then stop for fresh production authorization before Task 5. A subagent-driven approach is possible for repository-only tasks, but must not be used for the production phase without the user explicitly requesting delegated execution.
