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
