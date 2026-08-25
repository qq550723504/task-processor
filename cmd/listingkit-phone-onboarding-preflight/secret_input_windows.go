//go:build windows

package main

import (
	"os"

	"golang.org/x/term"
)

func openSecretInput(input *os.File) (*os.File, func(), error) {
	return selectSecretInput(input, term.IsTerminal, func() (*os.File, error) {
		return os.Open("CONIN$")
	})
}

func selectSecretInput(input *os.File, isTerminal func(int) bool, openConsole func() (*os.File, error)) (*os.File, func(), error) {
	if isTerminal(int(input.Fd())) {
		return input, func() {}, nil
	}
	console, err := openConsole()
	if err != nil {
		return nil, nil, err
	}
	return console, func() { _ = console.Close() }, nil
}
