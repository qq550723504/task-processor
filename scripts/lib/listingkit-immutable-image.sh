#!/usr/bin/env bash

# listingkit_is_immutable_image accepts the image forms that the two ListingKit
# release drivers can safely render into Kubernetes manifests. It is deliberately
# shared so the preflight and deployment gates cannot disagree on a candidate.
listingkit_is_immutable_image() {
  local image="${1:-}"
  [[ -n "$image" && "$image" != "latest" && "$image" != *:latest && "$image" =~ ^[A-Za-z0-9][A-Za-z0-9._:/-]*(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[A-Fa-f0-9]{64})$ ]]
}
