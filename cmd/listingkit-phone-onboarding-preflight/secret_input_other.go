//go:build !windows

package main

import "os"

func openSecretInput(input *os.File) (*os.File, func(), error) {
	return input, func() {}, nil
}
