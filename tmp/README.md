# tmp

This directory is for local probes, integration run output, and scratch files.
It is intentionally a nested Go module so root `go test ./...` does not treat
temporary probes as part of the main repository package graph.
