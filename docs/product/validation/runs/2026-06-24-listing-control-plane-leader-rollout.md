# Listing Control Plane Leader Rollout Validation

## Metadata

- Date: 2026-06-24
- Namespace: `task-processor`
- Deployment: `shein-listing-control-plane`
- Final image: `xuwei190/task-processor-listing-control-plane:fde9f803`
- Final source commit: `fde9f803`
- Final production replicas after validation: `1`

## Goal

Validate that the Go Listing Control Plane can run with two Kubernetes replicas without duplicate recovery or dispatch execution.

Expected behavior:

- exactly one pod acquires `listing:control-plane:leader:shein`;
- the leader executes recovery and dispatch;
- the standby pod remains Kubernetes-ready;
- the standby pod does not run recovery or dispatch;
- the deployment can be returned to one replica after the temporary validation.

## Validation timeline

### Attempt 1: old image exposed deployment drift

The first two-replica test used the live deployment before applying the latest manifest and image.

Observed:

- image was still `xuwei190/task-processor-listing-control-plane:c01378e1`;
- leader lock env was absent from the live deployment;
- `/status` did not expose `leader`;
- both pods emitted `listing control-plane cycle completed`.

Action:

- restored replicas to `1`;
- built and deployed `xuwei190/task-processor-listing-control-plane:d5974106`;
- applied the manifest containing leader lock env and pod identity wiring.

Root cause:

The source and manifest had been updated, but the live control-plane deployment had not yet been rolled to a leader-lock-capable image/config.

### Attempt 2: standby readiness gap

After deploying `d5974106`, the leader lock behavior worked, but rollout did not complete with two replicas.

Observed:

- leader pod status: `status=ok`, `ready=true`, `leader.isLeader=true`;
- standby pod status: `status=standby`, `ready=false`, `leader.isLeader=false`;
- standby pod logs repeatedly showed `listing control-plane standby; leader lock is held by another instance`;
- standby dispatch summary remained zero;
- Kubernetes rollout timed out because standby readiness returned unavailable.

Action:

- restored replicas to `1`;
- changed standby status semantics so standby is healthy and ready while still not executing dispatch/recovery;
- added tests covering standby readiness;
- built and deployed `xuwei190/task-processor-listing-control-plane:fde9f803`.

Root cause:

`RecordStandby` treated a healthy standby as not ready. That is correct for "cannot serve traffic" services, but wrong for an HA control-plane replica where standby is an intended healthy state.

## Final validation result

Command sequence:

```powershell
kubectl scale deployment shein-listing-control-plane -n task-processor --replicas=2
kubectl rollout status deployment/shein-listing-control-plane -n task-processor --timeout=180s
kubectl get pods -n task-processor -l app=shein-listing-control-plane -o wide
```

Result:

```text
deployment "shein-listing-control-plane" successfully rolled out
shein-listing-control-plane-c4d6d9c5-68xd9   1/1 Running
shein-listing-control-plane-c4d6d9c5-srksl   1/1 Running
```

Observed standby status:

```text
status=standby
ready=true
leader.owner=shein-listing-control-plane-c4d6d9c5-srksl-1
leader.isLeader=false
dispatch.candidates=0
dispatch.dispatched=0
```

Observed leader status:

```text
status=ok
ready=true
leader.owner=shein-listing-control-plane-c4d6d9c5-srksl-1
leader.isLeader=true
dispatch.candidates=10
dispatch.dispatched=1
dispatch.skipped=9
```

Observed standby logs:

```text
listing control-plane standby; leader lock is held by another instance
```

Observed leader logs:

```text
listing control-plane cycle completed
```

Final action:

```powershell
kubectl scale deployment shein-listing-control-plane -n task-processor --replicas=1
kubectl rollout status deployment/shein-listing-control-plane -n task-processor --timeout=120s
```

Final state:

```text
shein-listing-control-plane-c4d6d9c5-68xd9   1/1 Running
```

## Conclusion

Pass.

The Redis leader lease prevents duplicate control-plane execution under a two-replica rollout. A standby pod stays Kubernetes-ready and observable, but does not run recovery or dispatch.

Production remains at one replica after this validation. Moving the steady-state deployment to two replicas is now a capacity/availability decision rather than a correctness blocker.

## Remaining follow-ups

- Validate leader takeover after actively deleting the leader pod, not only after old-owner TTL expiry during rollout.
- Persist dispatch skip/delay reasons on tasks or append-only task events.
- Add daily limit / in-flight capacity into store runtime capacity calculation.
- Rehearse rollback from Go control plane to the documented fallback path.
