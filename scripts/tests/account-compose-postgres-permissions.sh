#!/bin/sh
# Integration regression for a NEW account-compose project with the image overlay initialized.
# Uses real TCP authentication and PostgreSQL enforcement, not configuration matching.
set -eu
umask 077
commercial=${ACCOUNT_COMMERCIAL_DATABASE:?}
databases="source_accounts $commercial referrals membership product_acquisition image_agent postgres template1"
denied=0
roles=0
check_role() {
  role=$1; own=$2; secret=$3
  PGPASSWORD=$(cat "$secret")
  export PGPASSWORD
  safe=$(psql -X -h 127.0.0.1 -p 5433 -U "$role" -d "$own" -At -v ON_ERROR_STOP=1 -c "SELECT NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_auth_members WHERE member=r.oid) FROM pg_roles r WHERE rolname=current_user")
  test "$safe" = t
  for db in $databases; do
    if [ "$db" = "$own" ]; then continue; fi
    if psql -X -h 127.0.0.1 -p 5433 -U "$role" -d "$db" -v VERBOSITY=verbose -v ON_ERROR_STOP=1 -c 'SELECT 1' >/tmp/compose-permissions-denied 2>&1; then
      echo "FAIL unexpected cross-database access: $role -> $db"; exit 1
    fi
    grep -Eq '42501|does not have CONNECT privilege' /tmp/compose-permissions-denied
    denied=$((denied+1))
  done
  case "$role" in *_runtime)
    for sql in 'BEGIN; CREATE TABLE public.compose_forbidden(id int); ROLLBACK;' 'BEGIN; CREATE TEMP TABLE compose_forbidden(id int); ROLLBACK;' 'SET ROLE business_cluster_admin'; do
      if psql -X -h 127.0.0.1 -p 5433 -U "$role" -d "$own" -v VERBOSITY=verbose -v ON_ERROR_STOP=1 -c "$sql" >/tmp/compose-permissions-denied 2>&1; then echo "FAIL elevated runtime privileges: $role"; exit 1; fi
      grep -Eq '42501|does not have CONNECT privilege' /tmp/compose-permissions-denied
      denied=$((denied+1))
    done
    ;;
  esac
  correct_password=$PGPASSWORD
  export PGPASSWORD=definitely-not-the-generated-password
  if psql -X -h 127.0.0.1 -p 5433 -U "$role" -d "$own" -c 'SELECT 1' >/tmp/compose-permissions-denied 2>&1; then echo "FAIL loopback authentication bypass: $role"; exit 1; fi
  grep -q 'password authentication failed' /tmp/compose-permissions-denied
  # The independent identity instance must not admit business credentials either.
  export PGPASSWORD=$correct_password
  if psql -X -h 127.0.0.1 -p 5432 -U postgres -d zitadel -c 'SELECT 1' >/tmp/compose-permissions-denied 2>&1; then echo "FAIL identity authentication bypass: $role"; exit 1; fi
  grep -q 'password authentication failed' /tmp/compose-permissions-denied
  roles=$((roles+1))
  echo "PASS $role: own database, non-superuser, no memberships, cross-database denial"
}
check_role source_account_owner source_accounts /secrets/source-owner/source-db-password
check_role commercial_schema_owner "$commercial" /secrets/commercial-owner/commercial-db-password
check_role referral_owner referrals /secrets/referral-owner/referral-db-password
check_role membership_owner membership /secrets/membership-owner/membership-db-password
check_role acquisition_owner product_acquisition /secrets/acquisition-owner/acquisition-db-password
check_role image_agent_owner image_agent /secrets/image-owner/image-db-password
check_role source_account_runtime source_accounts /secrets/source-runtime/source-runtime-password
check_role commercial_runtime "$commercial" /secrets/commercial-runtime/commercial-reader-password
check_role commercial_owner_runtime "$commercial" /secrets/commercial-runtime/commercial-owner-password
check_role referral_runtime referrals /secrets/referral-runtime/referral-runtime-password
check_role organization_membership_runtime membership /secrets/membership-runtime/membership-runtime-password
check_role source_acquisition_runtime product_acquisition /secrets/acquisition-runtime/acquisition-runtime-password
check_role image_agent_runtime image_agent /secrets/image-runtime/image-runtime-password
check_role image_agent_worker_runtime image_agent /secrets/image-worker/image-worker-password
PGPASSWORD=$(cat /secrets/commercial-runtime/commercial-reader-password)
export PGPASSWORD
for sql in 'UPDATE public.saas_plans SET active=false WHERE false' 'SELECT * FROM public.commercial_orders LIMIT 0' 'SELECT * FROM public.saas_modules LIMIT 0' 'SET ROLE commercial_owner_runtime'; do
  if psql -X -h 127.0.0.1 -p 5433 -U commercial_runtime -d "$commercial" -v VERBOSITY=verbose -v ON_ERROR_STOP=1 -c "$sql" >/tmp/compose-permissions-denied 2>&1; then echo 'FAIL commercial reader escaped boundary'; exit 1; fi
  grep -Eq '42501|does not have CONNECT privilege' /tmp/compose-permissions-denied
  denied=$((denied+1))
done
if psql -X -h 127.0.0.1 -p 5433 -U commercial_owner_runtime -d "$commercial" -c 'SELECT 1' >/tmp/compose-permissions-denied 2>&1; then echo 'FAIL commercial role passwords are shared'; exit 1; fi
grep -q 'password authentication failed' /tmp/compose-permissions-denied
echo "PASS role checks=$roles, permission denials=$denied, separate commercial passwords"
