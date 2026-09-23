package batchcapture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadFixture reads one sanitized local page. The fixtures contain no account
// data and the test never performs a network request; this is the evidence the
// design's section 17 clause 8 permits while real risk-control confirmation
// stays NOT_RUN.
func loadFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(raw)
}

// TestClassifyWithFixtures drives the classifier with the four page shapes
// required by design section 10 ("4-fixture classifier").
func TestClassifyWithFixtures(t *testing.T) {
	cases := []struct {
		name       string
		fixture    string
		finalURL   string
		hasProduct bool
		want       Verdict
	}{
		{
			name:     "login redirect page",
			fixture:  "challenge-login-redirect.html",
			finalURL: "https://login.1688.com/member/signin.htm",
			want:     VerdictPauseBatch,
		},
		{
			name:     "risk control page",
			fixture:  "challenge-risk-control.html",
			finalURL: "https://detail.1688.com/offer/1.html",
			want:     VerdictPauseBatch,
		},
		{
			name:     "200 page with no product and no marker",
			fixture:  "challenge-empty-200.html",
			finalURL: "https://detail.1688.com/offer/1.html",
			want:     VerdictPauseBatch,
		},
		{
			name:     "definitively delisted page",
			fixture:  "challenge-delisted.html",
			finalURL: "https://detail.1688.com/offer/1.html",
			want:     VerdictSingleItemFailure,
		},
		{
			name:       "product data present",
			fixture:    "challenge-empty-200.html",
			finalURL:   "https://detail.1688.com/offer/1.html",
			hasProduct: true,
			want:       VerdictProceed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Observation{
				FinalURL:           tc.finalURL,
				BodyText:           loadFixture(t, tc.fixture),
				ProductDataPresent: tc.hasProduct,
				Performed:          true,
			})
			if got != tc.want {
				t.Fatalf("fixture %s: want %s, got %s", tc.fixture, tc.want, got)
			}
		})
	}
}

// TestChallengeBeatsProductData proves the challenge judgment cannot be
// overridden by observing product data, which is the fail-closed direction.
func TestChallengeBeatsProductData(t *testing.T) {
	got := Classify(Observation{
		FinalURL:           "https://detail.1688.com/offer/1.html",
		BodyText:           loadFixture(t, "challenge-risk-control.html"),
		ProductDataPresent: true,
		Performed:          true,
	})
	if got != VerdictPauseBatch {
		t.Fatalf("a challenge page must stop the batch, got %s", got)
	}
}

// TestDelistedBeatsNothingButIsNotAChallenge covers the distinction the tenth
// review round required: a delisted product and a challenge page must not be
// conflated, because only one of them stops the batch.
func TestDelistedBeatsNothingButIsNotAChallenge(t *testing.T) {
	body := loadFixture(t, "challenge-delisted.html")
	if IsChallengePage("https://detail.1688.com/offer/1.html", body) {
		t.Fatal("a delisted page is not a challenge page")
	}
	verdict := Classify(Observation{
		FinalURL:  "https://detail.1688.com/offer/1.html",
		BodyText:  body,
		Performed: true,
	})
	if verdict != VerdictSingleItemFailure {
		t.Fatalf("want single-item failure, got %s", verdict)
	}
	action := verdict.Action()
	if action.StopBatch {
		t.Fatal("a single-item failure must not stop the batch")
	}
	if !action.ContinueToNextItem {
		t.Fatal("a single-item failure must continue to the next item")
	}
}

// TestUnperformedObservationFailsClosed covers "I did not look" never being read
// as a decision.
func TestUnperformedObservationFailsClosed(t *testing.T) {
	verdict := Classify(Observation{})
	if verdict != VerdictIndeterminate {
		t.Fatalf("want indeterminate, got %s", verdict)
	}
	action := verdict.Action()
	if !action.StopBatch || action.ContinueToNextItem {
		t.Fatalf("indeterminate must stop the batch and never skip: %+v", action)
	}
}

// TestPauseRedoesCurrentItem proves a paused item is neither skipped nor
// finalised: design section 4 D1.3 requires the same item to be re-done after a
// person clears the gate.
func TestPauseRedoesCurrentItem(t *testing.T) {
	action := VerdictPauseBatch.Action()
	if !action.StopBatch {
		t.Fatal("a pause must stop the batch")
	}
	if action.ContinueToNextItem {
		t.Fatal("a pause must never be treated as a single-item failure")
	}
	if !action.RedoCurrentItemAfterHuman {
		t.Fatal("a paused item must be re-done after human intervention")
	}
}

// TestRiskControlMarkerContract pins the measured marker set so a future edit
// cannot quietly widen or narrow the evidence without review.
func TestRiskControlMarkerContract(t *testing.T) {
	if len(riskControlMarkers) != 2 {
		t.Fatalf("expected the two measured markers, got %v", riskControlMarkers)
	}
	for _, marker := range riskControlMarkers {
		if marker != strings.ToLower(marker) {
			t.Fatalf("marker %q must be lowercase for case-insensitive matching", marker)
		}
	}
	if !HasRiskControlMarker("... BXPUNISH ...") {
		t.Fatal("matching must be case-insensitive")
	}
	if !HasRiskControlMarker("punish?x5secdata=abc") {
		t.Fatal("x5secdata must be recognised")
	}
	if HasRiskControlMarker("这是一张普通的商品页") {
		t.Fatal("an ordinary product page must not be read as a challenge")
	}
}

// TestFixtureCorpusIsSanitized asserts every fixture in testdata carries no
// account data or live hosts, so none of them can be mistaken for a recorded
// production page. The four challenge shapes are required to be present because
// design section 4 D1.3 fixes the observable contract at those four.
func TestFixtureCorpusIsSanitized(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	present := map[string]bool{}
	for _, entry := range entries {
		present[entry.Name()] = true
	}
	required := []string{
		"challenge-login-redirect.html",
		"challenge-risk-control.html",
		"challenge-empty-200.html",
		"challenge-delisted.html",
	}
	for _, name := range required {
		if !present[name] {
			t.Fatalf("missing required challenge fixture %s", name)
		}
	}
	// The product fixture the driver test serves must exist too, otherwise the
	// browser evidence silently disappears.
	if !present["batch-product.html"] {
		t.Fatal("missing batch-product.html, so the driver tests would have no product page")
	}
	for _, entry := range entries {
		body := loadFixture(t, entry.Name())
		for _, forbidden := range []string{"cn_logon", "_csrf_token", "last_mid", "cookie"} {
			if strings.Contains(strings.ToLower(body), forbidden) {
				t.Fatalf("fixture %s appears to contain session material %q", entry.Name(), forbidden)
			}
		}
	}
}

// TestRiskControlMarkerOutsideVisibleText is the regression test for the defect
// that the first fixture-driven run exposed: the measured 1688 risk-control pages
// carry their markers in element attributes and inline scripts. Visible text alone
// therefore never contains a marker, and a classifier that reads only visible text
// reports "proceed" for a page that is actually a verification gate.
func TestRiskControlMarkerOutsideVisibleText(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "challenge-risk-control.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	// The fixture's visible text deliberately contains no marker.
	if HasRiskControlMarker("为了您的账号安全，请完成验证。") {
		t.Fatal("fixture visible text must not itself contain a marker")
	}
	// The same observation is a pause once the page source is supplied.
	verdict := Classify(Observation{
		FinalURL:           "https://detail.1688.com/offer/981645030344.html",
		BodyText:           "为了您的账号安全，请完成验证。",
		RawHTML:            string(raw),
		ProductDataPresent: true,
		Performed:          true,
	})
	if verdict != VerdictPauseBatch {
		t.Fatalf("challenge in page source classified as %q, want pause_batch", verdict)
	}
	// And without the page source the same visible text alone must not pause,
	// which is exactly why the source is required.
	blind := Classify(Observation{
		FinalURL:           "https://detail.1688.com/offer/981645030344.html",
		BodyText:           "为了您的账号安全，请完成验证。",
		ProductDataPresent: true,
		Performed:          true,
	})
	if blind != VerdictProceed {
		t.Fatalf("visible-text-only observation classified as %q, want proceed", blind)
	}
}
