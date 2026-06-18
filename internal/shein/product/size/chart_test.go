package size

import (
	"testing"

	"task-processor/internal/model"
)

func TestResolveShoeSizeFromChart(t *testing.T) {
	chart := &model.SizeChart{
		Headers: []string{"Brand Size", "US Size", "UK Size", "EU Size"},
		Rows: [][]string{
			{"7", "7", "4", "37"},
		},
	}

	got, ok := ResolveShoeSizeFromChart(chart, "7 Wide")
	if !ok {
		t.Fatal("ResolveShoeSizeFromChart() = not found, want found")
	}
	if got.System != SystemUS {
		t.Fatalf("System = %q, want %q", got.System, SystemUS)
	}
	if got.BaseSize != "7" {
		t.Fatalf("BaseSize = %q, want 7", got.BaseSize)
	}
	if got.Width != WidthWide {
		t.Fatalf("Width = %q, want %q", got.Width, WidthWide)
	}
}

func TestResolveShoeSizeFromChart_PrefersMatchedColumnSystem(t *testing.T) {
	chart := &model.SizeChart{
		Headers: []string{"BR Size", "US Size", "UK Size"},
		Rows: [][]string{
			{"35", "7", "4"},
		},
	}

	got, ok := ResolveShoeSizeFromChart(chart, "7")
	if !ok {
		t.Fatal("ResolveShoeSizeFromChart() = not found, want found")
	}
	if got.System != SystemUS {
		t.Fatalf("System = %q, want %q", got.System, SystemUS)
	}
}

func TestBuildShoeSizeCandidatesFromChart(t *testing.T) {
	chart := &model.SizeChart{
		Headers: []string{"Brand Size", "US Size", "UK Size", "EU Size"},
		Rows: [][]string{
			{"10", "10", "7", "40"},
		},
	}

	got := BuildShoeSizeCandidatesFromChart(chart, "10 Wide")
	wantFirst := []string{"10 Wide", "US10 Wide", "US10W", "10", "US10", "7", "UK7", "40", "EU40"}
	for _, want := range wantFirst {
		found := false
		for _, candidate := range got {
			if candidate == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("candidates = %#v, want to contain %q", got, want)
		}
	}
}
