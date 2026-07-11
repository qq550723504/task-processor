package referenceanalysis

import (
	"errors"
	"testing"
)

func TestInterpretRejectsEmptyInput(t *testing.T) {
	_, err := Interpret(nil)
	if !errors.Is(err, ErrNoInput) {
		t.Fatalf("Interpret(nil) error = %v, want ErrNoInput", err)
	}
}

func TestInterpretRejectsWhitespaceOnlyInput(t *testing.T) {
	_, err := Interpret([]string{"  ", "\n"})
	if !errors.Is(err, ErrNoInput) {
		t.Fatalf("Interpret(whitespace) error = %v, want ErrNoInput", err)
	}
}
