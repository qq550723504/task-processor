# Task Event V2 Migration

## Scope

`TaskEventV2` is the RabbitMQ contract for messages carrying a complete task
payload. It is produced by the application task submitter and the distributed
crawler client, and decoded by `app/task.MessageAdapter` before a consumer
constructs a model task.

The Listing Control Plane's `listingcontrol.DispatchPublisher` is deliberately
out of scope. Its `ListingDispatchSignal` contains only a task ID, and its
receiver loads the task from persistent state. It is neither a V2 producer nor
a legacy consumer for this migration.

## Wire schema

```json
{
  "schemaVersion": 2,
  "taskId": "430604922543791994",
  "sourcePlatform": "amazon",
  "targetPlatform": "shein",
  "traceId": "optional-trace-id",
  "metadata": {"optional": "string-value"},
  "payload": {
    "taskId": "430604922543791994",
    "sourcePlatform": "amazon",
    "targetPlatform": "shein",
    "tenantId": 1001,
    "storeId": 2001,
    "productId": "B001TEST"
  }
}
```

`schemaVersion`, the top-level string `taskId`, `sourcePlatform`, and
`targetPlatform` are required. The domain normalizer rejects missing values or
an unsupported schema version. The adapter also rejects conflicting duplicate
routing values in the complete payload. New producers only publish V2.

The pre-V2 flat `platform` field is decoded only in
`internal/app/task/message_adapter.go`. It is converted there to explicit
source/target values before the domain normalizer runs; no domain consumer uses
that ambiguous field.

## Routing safety

Crawler queue lookup no longer defaults unknown routes to `amazon.crawler`.
It returns the classified `UNKNOWN_CRAWLER_ROUTE` task error so a caller can
record, alert, or dead-letter the invalid route without sending work to the
wrong queue.

RabbitMQ acknowledgement remains owned by the existing task handler: it still
claims task state before calling the acknowledgement callback. V2 decoding does
not alter that order.

## Compatibility measurement and removal gate

For two released consumer versions, retain the adapter's legacy decode branch.
Every legacy decode increments the existing consumer Prometheus registry metric
`task_event_decoded_total{schema_version="legacy"}`. It is exposed from the
consumer `/metrics` endpoint through `internal/infra/metrics/ConsumerRegistry`.
Use `increase(task_event_decoded_total{schema_version="legacy"}[14d])` for the
legacy-event observation gate. The label is deliberately limited to the stable
`schema_version` value; queue, task ID, tenant, and store are not metric labels.

Track `legacy_task_event_consumers` from the Kubernetes workload inventory:
list every consumer deployment/statefulset image digest and release label, then
compare it with the release manifest that first contains V2 decoding. Keep the
dated inventory with the release checklist; it is the source of truth for the
zero-consumer condition.

Remove the legacy branch only when all of the following are true:

1. Two consumer release cycles containing V2 decoding have completed.
2. `legacy_task_event_consumers` is zero.
3. `legacy_task_event_observations` is zero for 14 consecutive full days,
   across every task and crawler queue.

The 14-day window restarts after any legacy observation or any consumer that
can still publish or require legacy task events is detected. This document
contains no credentials or environment-specific connection details.
