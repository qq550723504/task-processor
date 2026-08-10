#!/usr/bin/env bash

# listingkit_is_immutable_image accepts only content-addressed images. It is
# deliberately shared so the preflight and deployment gates cannot disagree on
# the exact candidate bytes.
listingkit_is_immutable_image() {
  local image="${1:-}"
  [[ -n "$image" && "$image" =~ ^[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[A-Fa-f0-9]{64}$ ]]
}
