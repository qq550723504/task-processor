package batchcapture

import (
	"os"
	"strings"
	"testing"
)

// A target snapshot that cannot be taken must not be reported as "no tabs existed".
//
// The previous item's application tab normally survives into the next item and still
// carries the previous item's idempotency key, so an empty snapshot would make it
// look like a tab the current handoff just created. AppPageByKey would then click the
// previous payload while the current item records its own key, which is an operation
// attributed to the wrong item (design section 4 D1.2 guard 1).
//
// The empty set is the one default that cannot be allowed here, so the call has to
// keep reporting the failure. The failure itself is not observable from a healthy
// browser session, which is why this asserts on the source: deleting the error return
// would otherwise only show up as a rare wrong publication.
func TestHandoffPreparationPropagatesASnapshotFailure(t *testing.T) {
	raw, err := os.ReadFile("popup.go")
	if err != nil {
		t.Fatalf("read popup.go: %v", err)
	}
	text := string(raw)

	const assignment = "existing, err := p.driver.pageTargetIDs()"
	if !strings.Contains(text, assignment) {
		t.Fatalf("popup.go no longer keeps the snapshot error in a variable: the call must not degrade to an empty snapshot")
	}
	if strings.Contains(text, "existing, _ :=") {
		t.Fatalf("popup.go discards the snapshot error: an enumeration failure would look like 'no tabs existed'")
	}
	// The captured error has to be returned, not merely named.
	rest := text[strings.Index(text, assignment):]
	if !strings.Contains(rest, "if err != nil {\n\t\treturn nil, err") {
		t.Fatalf("popup.go names the snapshot error but does not return it")
	}
}
