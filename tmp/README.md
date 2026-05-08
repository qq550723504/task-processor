# tmp

This directory is for local probes, integration run output, and scratch files.
It is intentionally a nested Go module so root `go test ./...` does not treat
temporary probes as part of the main repository package graph.

Go probe programs in this directory should use `package main` and start with
`//go:build ignore`. Do not add these probes to the root module or import them
from production packages; keep reusable fixtures under package-local `testdata`
instead.
