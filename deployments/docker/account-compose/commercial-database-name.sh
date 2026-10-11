#!/bin/sh
# Shared by bootstrap and the PostgreSQL init hook before provisioning.
commercial_database=${ACCOUNT_COMMERCIAL_DATABASE:-commercial}
case "$commercial_database" in
  ''|[!a-z]*|*[!a-z0-9_]*) echo 'invalid commercial database name' >&2; exit 1 ;;
  postgres|template0|template1|source_accounts|referrals|membership|product_acquisition|image_agent|product_agent|store_center|knowledge|notification_center|agent_customization|ai_projects|reports|tool_market|ecoservices)
    echo 'commercial database must have its own name' >&2; exit 1 ;;
esac
test "${#commercial_database}" -le 63 || { echo 'invalid commercial database name' >&2; exit 1; }
