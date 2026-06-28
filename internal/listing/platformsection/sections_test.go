package platformsection

import (
	"errors"
	"testing"
)

func TestBuildAllRunsBuildersInOrderAndStopsOnError(t *testing.T) {
	t.Parallel()

	var calls []string
	errBoom := errors.New("boom")

	err := BuildAll([]Builder{
		{
			Platform: "amazon",
			Build: func() error {
				calls = append(calls, "amazon")
				return nil
			},
		},
		{
			Platform: "shein",
			Build: func() error {
				calls = append(calls, "shein")
				return errBoom
			},
		},
		{
			Platform: "temu",
			Build: func() error {
				calls = append(calls, "temu")
				return nil
			},
		},
	})

	if err != errBoom {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
	if want := []string{"amazon", "shein"}; !equalStrings(calls, want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}

func TestBuildAllSkipsNilBuilders(t *testing.T) {
	t.Parallel()

	var calls []string
	err := BuildAll([]Builder{
		{Platform: "amazon"},
		{
			Platform: "shein",
			Build: func() error {
				calls = append(calls, "shein")
				return nil
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"shein"}; !equalStrings(calls, want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
