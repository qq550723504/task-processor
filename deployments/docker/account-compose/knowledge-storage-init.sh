#!/bin/sh
set -eu
umask 077
export MC_CONFIG_DIR=/tmp/knowledge-mc
mc alias set local http://knowledge-objects:9000 knowledge_local_root "$(cat /private/root-password)" >/dev/null
mc mb --ignore-existing local/knowledge >/dev/null
mc anonymous set none local/knowledge >/dev/null
mc admin policy create local knowledge-runtime /init/policy.json >/dev/null
mc admin user add local "$(cat /private/access-key)" "$(cat /private/secret-key)" >/dev/null
mc admin policy attach local knowledge-runtime --user "$(cat /private/access-key)" >/dev/null
