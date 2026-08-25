//go:build windows

package main

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectSecretInputUsesConsoleInputWhenStandardInputIsNotTerminal(t *testing.T) {
	stdin, stdinWriter, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, stdin.Close())
		require.NoError(t, stdinWriter.Close())
	})

	console, consoleWriter, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = console.Close()
		_ = consoleWriter.Close()
	})

	opened := false
	got, closeInput, err := selectSecretInput(stdin, func(int) bool { return false }, func() (*os.File, error) {
		opened = true
		return console, nil
	})

	require.NoError(t, err)
	require.True(t, opened)
	require.Same(t, console, got)
	closeInput()
}

func TestSelectSecretInputReturnsConsoleOpenErrorWhenStandardInputIsNotTerminal(t *testing.T) {
	stdin, stdinWriter, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, stdin.Close())
		require.NoError(t, stdinWriter.Close())
	})

	want := errors.New("CONIN$ unavailable")
	_, _, err = selectSecretInput(stdin, func(int) bool { return false }, func() (*os.File, error) {
		return nil, want
	})

	require.ErrorIs(t, err, want)
}

func TestSelectSecretInputKeepsTerminalStandardInput(t *testing.T) {
	stdin, stdinWriter, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, stdin.Close())
		require.NoError(t, stdinWriter.Close())
	})

	got, closeInput, err := selectSecretInput(stdin, func(int) bool { return true }, func() (*os.File, error) {
		return nil, errors.New("console input should not be opened")
	})

	require.NoError(t, err)
	require.Same(t, stdin, got)
	closeInput()
}
