package extractor

import "testing"

func TestParseSizeChartFromBodyText(t *testing.T) {
	bodyText := `
Color:
Black
Size:
Select 5 5.5 6 6.5 7
Purchase options and add-ons
Size Chart
US WIDTH (M)
Brand Size US Size UK Size EU Size
5 5 2 35
5.5 5.5 2.5 35.5
6 6 3 36
6.5 6.5 3.5 36.5
7 7 4 37
Product details
Fabric type
100% mesh
`

	chart := parseSizeChartFromBodyText(bodyText)
	if chart == nil {
		t.Fatal("expected size chart to be parsed")
	}

	if chart.Title != "Size Chart" {
		t.Fatalf("expected title Size Chart, got %q", chart.Title)
	}

	if chart.Subtitle != "US WIDTH (M)" {
		t.Fatalf("expected subtitle US WIDTH (M), got %q", chart.Subtitle)
	}

	wantHeaders := []string{"Brand Size", "US Size", "UK Size", "EU Size"}
	if len(chart.Headers) != len(wantHeaders) {
		t.Fatalf("expected %d headers, got %d: %#v", len(wantHeaders), len(chart.Headers), chart.Headers)
	}
	for i, want := range wantHeaders {
		if chart.Headers[i] != want {
			t.Fatalf("expected header[%d]=%q, got %q", i, want, chart.Headers[i])
		}
	}

	if len(chart.Rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(chart.Rows))
	}

	wantFirstRow := []string{"5", "5", "2", "35"}
	for i, want := range wantFirstRow {
		if chart.Rows[0][i] != want {
			t.Fatalf("expected row[0][%d]=%q, got %q", i, want, chart.Rows[0][i])
		}
	}
}

func TestParseSizeChartFromBodyTextReturnsNilWhenMissing(t *testing.T) {
	bodyText := `
Color:
Black
Product details
Fabric type
100% mesh
`

	if chart := parseSizeChartFromBodyText(bodyText); chart != nil {
		t.Fatalf("expected nil chart, got %#v", chart)
	}
}

func TestParseSizeChartFromSourcesUsesPopoverWhenBodyOnlyHasTrigger(t *testing.T) {
	bodyText := `
Color:
Navy/White
Purchase options and add-ons
Size Chart
Product details
Fabric type
100% mesh
`

	popoverText := `
US WIDTH (M)
Brand Size US Size UK Size EU Size
5 5 2 35
5.5 5.5 2.5 35.5
6 6 3 36
6.5 6.5 3.5 36.5
`

	chart := parseSizeChartFromSources(bodyText, popoverText)
	if chart == nil {
		t.Fatal("expected size chart to be parsed from popover text")
	}

	if chart.Subtitle != "US WIDTH (M)" {
		t.Fatalf("expected subtitle US WIDTH (M), got %q", chart.Subtitle)
	}

	if len(chart.Rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(chart.Rows))
	}

	if chart.Rows[3][0] != "6.5" || chart.Rows[3][3] != "36.5" {
		t.Fatalf("unexpected last row: %#v", chart.Rows[3])
	}
}
