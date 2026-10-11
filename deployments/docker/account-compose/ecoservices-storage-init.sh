#!/bin/sh
set -eu
umask 077
export MC_CONFIG_DIR=/tmp/ecoservices-mc
mc alias set local http://127.0.0.1:9101 ecoservices_local_root "$(cat /ecoservices-private/root-password)" >/dev/null
mc mb --ignore-existing local/ecoservices >/dev/null
mc anonymous set none local/ecoservices >/dev/null
mc admin policy create local ecoservices-runtime /init/ecoservices-storage-policy.json >/dev/null
mc admin user add local "$(cat /object-runtime/access-key)" "$(cat /object-runtime/secret-key)" >/dev/null
mc admin policy attach local ecoservices-runtime --user "$(cat /object-runtime/access-key)" >/dev/null
echo 'Private ecosystem qualification object storage ready'
