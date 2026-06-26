package publishing

import "testing"

func TestShouldRetrySensitiveWordSubmitRequiresPublishErrorNotesAndExecutor(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		action              string
		hasResponse         bool
		hasResponseError    bool
		validationNoteCount int
		hasExecutor         bool
		want                bool
	}{
		"publish with response error notes and executor": {
			action:              "publish",
			hasResponse:         true,
			hasResponseError:    true,
			validationNoteCount: 1,
			hasExecutor:         true,
			want:                true,
		},
		"unnormalized publish action": {
			action:              " publish ",
			hasResponse:         true,
			hasResponseError:    true,
			validationNoteCount: 1,
			hasExecutor:         true,
		},
		"save draft": {
			action:              "save_draft",
			hasResponse:         true,
			hasResponseError:    true,
			validationNoteCount: 1,
			hasExecutor:         true,
		},
		"missing response": {
			action:              "publish",
			hasResponseError:    true,
			validationNoteCount: 1,
			hasExecutor:         true,
		},
		"missing response error": {
			action:              "publish",
			hasResponse:         true,
			validationNoteCount: 1,
			hasExecutor:         true,
		},
		"missing validation notes": {
			action:           "publish",
			hasResponse:      true,
			hasResponseError: true,
			hasExecutor:      true,
		},
		"missing executor": {
			action:              "publish",
			hasResponse:         true,
			hasResponseError:    true,
			validationNoteCount: 1,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := ShouldRetrySensitiveWordSubmit(tt.action, tt.hasResponse, tt.hasResponseError, tt.validationNoteCount, tt.hasExecutor)
			if got != tt.want {
				t.Fatalf("ShouldRetrySensitiveWordSubmit() = %v, want %v", got, tt.want)
			}
		})
	}
}
