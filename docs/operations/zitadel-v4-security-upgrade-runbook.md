# ZITADEL v4 Security Upgrade Runbook

Upgrade production ZITADEL and Login V2 from v4.13.1 to v4.17.1 before enabling new phone or external-IdP paths.

**Prerequisites:** explicit production authorization for the database snapshot, restore rehearsal, image update, and real-device login verification. Run `scripts/zitadel-v4-upgrade-preflight.ps1` first and record its JSON output.

## Acceptance checklist

Complete every item before declaring the upgrade done:

- [ ] Database backup identifier and isolated restore rehearsal timestamp are recorded without credentials.
- [ ] `zitadel` and `zitadel-login` are Ready on v4.17.1.
- [ ] `https://auth.shuomiai.com/.well-known/openid-configuration` returns 200.
- [ ] Incognito email/password and ListingKit callback login both work.
- [ ] OTP SMS and Generic OIDC remain disabled.

## Approved change sequence

Run only inside the approved change window, after the backup and restore-rehearsal evidence is recorded. ZITADEL requires its target-version `setup` phase before the new runtime starts; changing images alone leaves new schema/projection migrations unapplied.

Before setup, verify that the ZITADEL service role owns objects in the dedicated ZITADEL database. In the current cluster that role is `zitadel_app`; historical objects owned by the PostgreSQL administrator must be corrected only inside the `eventstore`, `projections`, and `system` schemas. Do not use a cluster-wide or cross-database ownership reassignment.

```bash
# Stop runtime processes so setup is the only process changing schema/projections.
kubectl -n zitadel scale deployment/zitadel --replicas=0
kubectl -n zitadel scale deployment/zitadel-login --replicas=0

# Update both images while their runtime replicas are stopped.
kubectl -n zitadel set image deployment/zitadel zitadel=ghcr.io/zitadel/zitadel:v4.17.1
kubectl -n zitadel set image deployment/zitadel-login zitadel-login=ghcr.io/zitadel/zitadel-login:v4.17.1

# Clone only the existing deployment references into an idempotent setup Job.
# The generator executes: setup --masterkeyFromEnv --init-projections=true
pwsh -File scripts/zitadel-v4-upgrade-setup.ps1 -JobName zitadel-v4-setup -Apply
kubectl -n zitadel wait --for=condition=complete job/zitadel-v4-setup --timeout=15m

# The public endpoint is served by Traefik, so ZITADEL runtime uses external TLS.
kubectl -n zitadel patch deployment zitadel --type=strategic --patch '{"spec":{"template":{"spec":{"containers":[{"name":"zitadel","args":["start","--masterkeyFromEnv","--tlsMode","external"]}]}}}}'
kubectl -n zitadel scale deployment/zitadel --replicas=1
kubectl -n zitadel rollout status deployment/zitadel --timeout=10m
curl --fail --silent --show-error https://auth.shuomiai.com/.well-known/openid-configuration >/dev/null
kubectl -n zitadel scale deployment/zitadel-login --replicas=1
kubectl -n zitadel rollout status deployment/zitadel-login --timeout=10m
```

Then re-run the preflight and confirm `upgradeRequired` is `false`.

## Failure boundary

If setup or a rollout fails, stop feature activation. Do not downgrade a migrated ZITADEL database in place. Restore the rehearsed backup into the isolated recovery target, validate there, and obtain a new approval before moving production traffic.
