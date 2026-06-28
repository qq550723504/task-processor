package platform

import (
	"errors"
	"testing"
)

func TestSectionBuildersExposeRegisteredPlatforms(t *testing.T) {
	t.Parallel()

	builders := SectionBuilders([]SectionRegistration[string, *string]{
		{Platform: "amazon", Build: func(context string, target *string, selectedPlatform string) error { return nil }},
		{Platform: "shein", Build: func(context string, target *string, selectedPlatform string) error { return nil }},
	})

	if len(builders) != 2 {
		t.Fatalf("SectionBuilders() length = %d, want 2", len(builders))
	}
	if got := builders[0].Platform(); got != "amazon" {
		t.Fatalf("builders[0].Platform() = %q, want amazon", got)
	}
	if got := builders[1].Platform(); got != "shein" {
		t.Fatalf("builders[1].Platform() = %q, want shein", got)
	}
}

func TestBuildRegisteredSectionsRunsInOrderAndStopsOnError(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	var calls []string
	target := "target"

	builders := SectionBuilders([]SectionRegistration[string, *string]{
		{
			Platform: "amazon",
			Build: func(context string, target *string, selectedPlatform string) error {
				calls = append(calls, context+":amazon:"+*target+":"+selectedPlatform)
				return nil
			},
		},
		{
			Platform: "shein",
			Build: func(context string, target *string, selectedPlatform string) error {
				calls = append(calls, context+":shein:"+*target+":"+selectedPlatform)
				return errBoom
			},
		},
		{
			Platform: "temu",
			Build: func(context string, target *string, selectedPlatform string) error {
				calls = append(calls, context+":temu:"+*target+":"+selectedPlatform)
				return nil
			},
		},
	})

	err := BuildRegisteredSections(builders, "result", &target, "shein")
	if err != errBoom {
		t.Fatalf("BuildRegisteredSections() error = %v, want %v", err, errBoom)
	}
	if want := []string{"result:amazon:target:shein", "result:shein:target:shein"}; !equalStrings(calls, want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}
