package extractor

import (
	"regexp"
	"strings"
	"task-processor/internal/model"

	"github.com/mxschmitt/playwright-go"
)

var sizeChartHeaderPattern = regexp.MustCompile(`[A-Za-z/]+\s+Size`)
var sizeChartTriggerSelector = `a.a-popover-trigger:has-text("Size Chart")`
var sizeChartPopoverSelector = `#a-popover-content-1, .fit-sizechartv2-popover-content`

// SizeChartExtractor Amazon 尺码表提取器
type SizeChartExtractor struct{}

func NewSizeChartExtractor() *SizeChartExtractor {
	return &SizeChartExtractor{}
}

func (e *SizeChartExtractor) Extract(page playwright.Page, product *model.Product) error {
	bodyText, err := page.Evaluate(`() => document.body ? (document.body.innerText || "") : ""`)
	if err != nil {
		return err
	}

	bodyString, _ := bodyText.(string)
	popoverText, err := extractSizeChartPopoverText(page)
	if err != nil {
		return err
	}

	product.SizeChart = parseSizeChartFromSources(bodyString, popoverText)
	return nil
}

func parseSizeChartFromSources(bodyText, popoverText string) *model.SizeChart {
	if chart := parseSizeChartFromBodyText(bodyText); chart != nil {
		return chart
	}

	popoverText = strings.TrimSpace(popoverText)
	if popoverText == "" {
		return nil
	}

	if !strings.Contains(strings.ToLower(popoverText), "size chart") {
		popoverText = "Size Chart\n" + popoverText
	}
	return parseSizeChartFromBodyText(popoverText)
}

func parseSizeChartFromBodyText(bodyText string) *model.SizeChart {
	if strings.TrimSpace(bodyText) == "" {
		return nil
	}

	lines := normalizeNonEmptyLines(bodyText)
	start := indexOfLine(lines, "Size Chart")
	if start < 0 || start+1 >= len(lines) {
		return nil
	}

	section := collectSizeChartSection(lines[start:])
	if len(section) < 3 {
		return nil
	}

	chart := &model.SizeChart{
		Title:   section[0],
		RawText: strings.Join(section, "\n"),
	}

	cursor := 1
	if cursor < len(section) && !looksLikeHeaderLine(section[cursor]) {
		chart.Subtitle = section[cursor]
		cursor++
	}
	if cursor >= len(section) {
		return nil
	}

	headers := parseSizeChartHeaders(section[cursor])
	if len(headers) == 0 {
		return nil
	}
	chart.Headers = headers
	cursor++

	for ; cursor < len(section); cursor++ {
		row := parseSizeChartRow(section[cursor], len(headers))
		if len(row) == len(headers) {
			chart.Rows = append(chart.Rows, row)
		}
	}

	if len(chart.Rows) == 0 {
		return nil
	}
	return chart
}

func normalizeNonEmptyLines(bodyText string) []string {
	rawLines := strings.Split(bodyText, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		normalized := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if normalized == "" {
			continue
		}
		lines = append(lines, normalized)
	}
	return lines
}

func indexOfLine(lines []string, target string) int {
	for i, line := range lines {
		if strings.EqualFold(line, target) {
			return i
		}
	}
	return -1
}

func collectSizeChartSection(lines []string) []string {
	stopLines := map[string]struct{}{
		"Product details":           {},
		"About this item":           {},
		"Customer reviews":          {},
		"Customers say":             {},
		"Looking for specific info": {},
	}

	section := make([]string, 0, len(lines))
	for i, line := range lines {
		if i > 0 {
			if _, shouldStop := stopLines[line]; shouldStop {
				break
			}
		}
		section = append(section, line)
	}
	return section
}

func looksLikeHeaderLine(line string) bool {
	if strings.Count(line, "Size") >= 2 {
		return true
	}
	return len(sizeChartHeaderPattern.FindAllString(line, -1)) >= 2
}

func parseSizeChartHeaders(line string) []string {
	headers := sizeChartHeaderPattern.FindAllString(line, -1)
	if len(headers) > 0 {
		return headers
	}
	return nil
}

func parseSizeChartRow(line string, expectedColumns int) []string {
	fields := strings.Fields(line)
	if len(fields) != expectedColumns {
		return nil
	}
	return fields
}

func extractSizeChartPopoverText(page playwright.Page) (string, error) {
	if page == nil {
		return "", nil
	}

	trigger := page.Locator(sizeChartTriggerSelector).First()
	count, err := trigger.Count()
	if err != nil || count == 0 {
		return "", err
	}

	if err := trigger.Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(5000),
	}); err != nil {
		return "", nil
	}

	popover := page.Locator(sizeChartPopoverSelector).First()
	if err := popover.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	}); err != nil {
		return "", nil
	}

	text, err := popover.Evaluate(`el => el ? (el.innerText || el.textContent || "") : ""`, nil)
	if err != nil {
		return "", nil
	}
	textString, _ := text.(string)
	return textString, nil
}
