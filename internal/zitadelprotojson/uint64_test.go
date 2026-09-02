package zitadelprotojson

import (
	"encoding/json"
	"testing"
)

func TestUint64AcceptsProtoJSONAndNumericCompatibilityForms(t *testing.T) {
	for _, test := range []struct {
		name string
		json string
		want uint64
	}{
		{name: "quoted", json: `"18446744073709551615"`, want: ^uint64(0)},
		{name: "numeric", json: `42`, want: 42},
	} {
		t.Run(test.name, func(t *testing.T) {
			var value Uint64
			if err := json.Unmarshal([]byte(test.json), &value); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if uint64(value) != test.want {
				t.Fatalf("Unmarshal() = %d, want %d", value, test.want)
			}
		})
	}
}

func TestUint64RejectsNonCanonicalOrOutOfRangeValues(t *testing.T) {
	for _, input := range []string{
		`"18446744073709551616"`,
		`-1`,
		`1.5`,
		`"1.5"`,
		`""`,
	} {
		t.Run(input, func(t *testing.T) {
			var value Uint64
			if err := json.Unmarshal([]byte(input), &value); err == nil {
				t.Fatalf("Unmarshal(%s) unexpectedly succeeded with %d", input, value)
			}
		})
	}
}

func TestUint64UnmarshalJSONRejectsNullWithoutTreatingItAsZero(t *testing.T) {
	// Mutation caught: replacing strict token validation with json.Unmarshal
	// directly into uint64 accepts null as zero.
	for _, test := range []struct {
		name    string
		initial Uint64
	}{
		{name: "fresh receiver", initial: 0},
		{name: "non-zero receiver", initial: 42},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := test.initial

			err := json.Unmarshal([]byte(`null`), &value)

			if err == nil {
				t.Fatalf("Unmarshal(null) unexpectedly succeeded with %d", value)
			}
			if value != test.initial {
				t.Fatalf("Unmarshal(null) changed receiver to %d, want %d", value, test.initial)
			}
		})
	}
}

func TestUint64UnmarshalJSONAcceptsCanonicalDirectTokens(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  Uint64
	}{
		{name: "zero", input: `0`, want: 0},
		{name: "positive decimal", input: `42`, want: 42},
		{name: "maximum uint64", input: `18446744073709551615`, want: ^Uint64(0)},
		{name: "quoted decimal compatibility", input: `"00042"`, want: 42},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := Uint64(9)

			if err := value.UnmarshalJSON([]byte(test.input)); err != nil {
				t.Fatalf("UnmarshalJSON(%s) error = %v", test.input, err)
			}
			if value != test.want {
				t.Fatalf("UnmarshalJSON(%s) = %d, want %d", test.input, value, test.want)
			}
		})
	}
}

func TestUint64UnmarshalJSONRejectsInvalidNumericLexemesWithoutMutatingReceiver(t *testing.T) {
	// Mutation caught: accepting all digit-only unquoted tokens permits JSON
	// numbers that are not complete canonical JSON unsigned integers.
	for _, test := range []struct {
		name  string
		input string
	}{
		{name: "leading zero", input: `01`},
		{name: "multiple leading zeroes", input: `00`},
		{name: "leading zero before positive decimal", input: `042`},
		{name: "trailing numeric token", input: `42 0`},
		{name: "trailing null token", input: `42 null`},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := Uint64(9)

			err := value.UnmarshalJSON([]byte(test.input))

			if err == nil {
				t.Fatalf("UnmarshalJSON(%s) unexpectedly succeeded with %d", test.input, value)
			}
			if value != 9 {
				t.Fatalf("UnmarshalJSON(%s) changed receiver to %d, want 9", test.input, value)
			}
		})
	}
}
