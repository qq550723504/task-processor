#!/bin/sh
# Called by the existing schema owner before the fresh-install complete marker.
set -eu
umask 077
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
test ! -f /schema-state/.init-complete || { echo 'data services require a new empty Product installation' >&2; exit 1; }
for path in /secrets/acquisition-owner/acquisition-db-password /secrets/acquisition-runtime/acquisition-runtime-password /secrets/data-services-runtime/password /runtime/current-application.json; do
 test -s "$path" || { echo 'data services installation inputs unavailable' >&2; exit 1; }
done
jq -n --rawfile password /secrets/acquisition-owner/acquisition-db-password \
 '{schemaVersion:1,collections:true,dataServices:true,database:{host:"127.0.0.1",port:5433,user:"acquisition_owner",password:($password|rtrimstr("\n")),database:"product_acquisition"}}' > "$work/product.json"
product-acquisition-init -config "$work/product.json" -confirm-empty-database product_acquisition
jq --rawfile productPassword /secrets/acquisition-runtime/acquisition-runtime-password --rawfile dataPassword /secrets/data-services-runtime/password \
 '.productCollections=true |
 .productAcquisitionDatabase={host:"127.0.0.1",port:5433,user:"source_acquisition_runtime",password:($productPassword|rtrimstr("\n")),database:"product_acquisition",maxConnections:4} |
 .dataServices={database:{host:"127.0.0.1",port:5433,user:"data_services_runtime",password:($dataPassword|rtrimstr("\n")),database:"product_acquisition",maxConnections:4},temporalAddress:"127.0.0.1:7233",temporalNamespace:"default",browserExecutable:"/opt/data-services/chromium",driverDirectory:"/opt/data-services/playwright",enabledSites:["us","uk","de","fr","it","es","ca","jp","au","mx","br","in","ae","sa"],trustedProxyCIDRs:["127.0.0.1/32","::1/128"]}' \
 /runtime/current-application.json > "$work/current.json"
chmod 600 "$work/current.json"
mv "$work/current.json" /runtime/current-application.json
