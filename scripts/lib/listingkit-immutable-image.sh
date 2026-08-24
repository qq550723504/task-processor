#!/usr/bin/env bash

# listingkit_is_immutable_image accepts only content-addressed images. It is
# deliberately shared so the preflight and deployment gates cannot disagree on
# the exact candidate bytes.
listingkit_is_immutable_image() {
  local image="${1:-}"
  [[ -n "$image" && "$image" =~ ^[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[A-Fa-f0-9]{64}$ ]]
}

# listingkit_compose_immutable_image reconstructs a content-addressed image
# from a repository and digest. Callers can safely pass only the digest across
# GitHub Actions job outputs, which avoids secret-output suppression when a
# registry namespace happens to match a masked credential.
listingkit_compose_immutable_image() {
  local repository="${1:-}"
  local digest="${2:-}"
  local image="${repository}@${digest}"

  listingkit_is_immutable_image "$image" || return 1
  printf '%s\n' "$image"
}
