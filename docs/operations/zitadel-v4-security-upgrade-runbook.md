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

Run only inside the approved change window, after the backup and restore-rehearsal evidence is recorded:

```bash
kubectl -n zitadel set image deployment/zitadel zitadel=ghcr.io/zitadel/zitadel:v4.17.1
kubectl -n zitadel set image deployment/zitadel zitadel-login=ghcr.io/zitadel/zitadel-login:v4.17.1
kubectl -n zitadel rollout status deployment/zitadel --timeout=10m
kubectl -n zitadel rollout status deployment/zitadel-login --timeout=10m
curl --fail --silent --show-error https://auth.shuomiai.com/.well-known/openid-configuration >/dev/null
```

Then re-run the preflight and confirm `upgradeRequired` is `false`.

## Failure boundary

If a rollout fails, stop feature activation. Do not downgrade a migrated ZITADEL database in place. Restore the rehearsed backup into the isolated recovery target, validate there, and obtain a new approval before moving production traffic.
