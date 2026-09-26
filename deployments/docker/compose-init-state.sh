require_current_init_marker() {
  marker=$1
  expected=$2
  if [ "$(cat "$marker")" != "$expected" ]; then
    echo 'schema state marker is stale; recreate this Compose project with the documented destroy command' >&2
    return 1
  fi
}

write_current_init_marker() {
  marker=$1
  version=$2
  temporary_marker="${marker}.tmp"
  printf '%s\n' "$version" > "$temporary_marker"
  chmod 600 "$temporary_marker"
  mv "$temporary_marker" "$marker"
}
