#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
library="$repo_root/scripts/lib/listingkit-immutable-image.sh"

# shellcheck source=../lib/listingkit-immutable-image.sh
source "$library"

repository='docker.io/xuwei190/task-processor-product-listing-api'
digest='sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
want="$repository@$digest"

if got="$(listingkit_compose_immutable_image "$repository" "$digest")"; then
  if [[ "$got" != "$want" ]]; then
    printf 'immutable image = %q, want %q\n' "$got" "$want" >&2
    exit 1
  fi
else
  printf '%s\n' 'expected a valid repository and digest to compose an immutable image' >&2
  exit 1
fi

for invalid_digest in '' 'sha256:not-a-digest' 'latest'; do
  if listingkit_compose_immutable_image "$repository" "$invalid_digest" >/dev/null; then
    printf 'invalid digest %q composed an immutable image\n' "$invalid_digest" >&2
    exit 1
  fi
done

printf '%s\n' 'ListingKit immutable image composition tests passed'
