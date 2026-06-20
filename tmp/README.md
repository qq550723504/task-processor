# tmp

Local one-off debug programs live here.

This directory is intentionally a separate Go module so `go test ./...` from the
main repository does not try to compile every ad-hoc `main` package under `tmp/`.
