package batchcapture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// receiverSourcePath is the production page this package drives.
const receiverSourcePath = "web/listingkit-ui/src/app/capture/1688/capture-receiver.tsx"

// The executor's half of the application-page contract is the selector list in
// apppage.go; the other half is rendered by the capture page. A rename on either side is
// silent: the driver simply never finds the node, and every real-path run stops with
// ErrSubmitUnavailable instead of saying which attribute went missing.
//
// This pins the two sides together with no browser, so the drift is caught by the Go
// suite that runs on every change. The behaviour of each attribute (which value it
// carries and when) is pinned by the component's own tests, which is where a DOM value
// belongs.
func TestTheCapturePageRendersEverySelectorTheDriverQueries(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(receiverSourcePath)))
	if err != nil {
		t.Fatalf("read the capture page the executor drives: %v", err)
	}
	page := string(raw)
	// The driver queries CSS attribute selectors; the page writes the bare attribute.
	for _, selector := range []string{selScopeActor, selScopeOrganization, selConfirmAndSubmit, selSubmitResult, selSubmitRefusal} {
		attribute := strings.TrimSuffix(strings.TrimPrefix(selector, "["), "]")
		if !strings.Contains(page, attribute) {
			t.Errorf("the capture page no longer renders %s, so the executor can only stop with ErrSubmitUnavailable", selector)
		}
	}
	// Not a selector: read off the result node, and required for every terminal status
	// (see readableTerminalResult), so its absence makes every result unreadable.
	if !strings.Contains(page, "data-batch-operation-id") {
		t.Error("the capture page no longer renders data-batch-operation-id, so no terminal result is recordable")
	}
}
