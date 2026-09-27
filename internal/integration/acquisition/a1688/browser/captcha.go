package browser

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// Captcha handling boundary (design A4, user decision 2026-09-26).
//
// The operator's legacy implementation contains a ~2800-line human-behavior
// engine. Only the minimal usable part is extracted here: locate the slider
// control, drag it once along an eased trajectory, then verify the page really
// left the challenge state. There is deliberately:
//
//   - no human-behavior simulation engine and no randomized "human" jitter,
//   - no image/click/text/math captcha solving,
//   - at most ONE automatic attempt per acquisition,
//   - no manual-intervention path: failure is reported honestly as
//     ErrChallenge and never dressed up as a collected product (D4).
var (
	// ErrCaptchaUnsupported means a challenge was detected but this build has no
	// automatic handling for its type.
	ErrCaptchaUnsupported = errors.New("captcha type not automatically handled")
)

// captchaAttemptBudget bounds the whole automatic handling step. It is far
// below the acquisition budget so a stuck challenge cannot consume it.
const captchaAttemptBudget = 20 * time.Second

// sliderSelectors are the challenge slider control selectors observed on 1688
// challenge pages (extracted from the legacy captcha handler).
var sliderSelectors = []string{
	".nc_iconfont.btn_slide",
	".btn_slide",
	"span.btn_slide",
	".slider-button",
	".captcha-slider-button",
	".geetest_slider_button",
	".geetest_slider",
	".verify-slider-button",
	".slide-verify-slider-mask-item",
	"[class*='slider'][class*='button']",
}

// challengeGoneSelectors indicate the challenge has been cleared.
var challengeGoneSelectors = []string{
	".slider-success",
	".verify-success",
	".nc_wrapper.is-success",
	"[class*='success'][class*='slider']",
}

// trySolveCaptcha makes at most one bounded automatic attempt to clear a
// detected challenge. It reports whether the page left the challenge state.
//
// A false return is not a failure of this function; the caller re-checks the
// page and reports ErrChallenge honestly if it is still challenged.
func trySolveCaptcha(ctx context.Context, page playwright.Page) (bool, error) {
	if page == nil {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, captchaAttemptBudget)
	defer cancel()

	button, err := firstVisible(ctx, page, sliderSelectors)
	if err != nil {
		return false, err
	}
	if button == nil {
		// A challenge we cannot act on (image/click/text) is reported as
		// unsupported rather than silently retried.
		return false, ErrCaptchaUnsupported
	}
	defer func() { _ = button.Dispose() }()

	box, err := button.BoundingBox()
	if err != nil || box == nil || box.Width <= 0 {
		return false, ErrCaptchaUnsupported
	}

	// A single eased drag. The trajectory is deterministic: this build does not
	// attempt to imitate a human, it only moves the control to the far end of its
	// track and lets the page decide.
	const steps = 24
	if err := dragAlong(ctx, page, box, steps); err != nil {
		return false, err
	}
	// Give the page a bounded moment to react to the drop.
	if err := waitForChallengeGone(ctx, page); err != nil {
		return false, err
	}
	return true, nil
}

// dragAlong moves the slider from its origin to the far end of the viewport
// width using a smoothstep easing, which is enough for the control to register
// a complete drag without imitating human timing.
func dragAlong(ctx context.Context, page playwright.Page, box *playwright.Rect, steps int) error {
	if steps <= 0 {
		steps = 1
	}
	startX := box.X + box.Width/2
	startY := box.Y + box.Height/2
	// Travel far enough to leave any plausible track; clamped to the viewport.
	viewport := page.ViewportSize()
	endX := float64(viewport.Width) - 10
	if endX <= startX {
		endX = startX + box.Width*4
	}

	mouse := page.Mouse()
	if err := mouse.Move(startX, startY); err != nil {
		return fmt.Errorf("%w: move: %v", ErrCaptchaUnsupported, err)
	}
	if err := mouse.Down(); err != nil {
		return fmt.Errorf("%w: down: %v", ErrCaptchaUnsupported, err)
	}
	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			_ = mouse.Up()
			return ctx.Err()
		default:
		}
		t := float64(i) / float64(steps)
		eased := t * t * (3 - 2*t) // smoothstep
		if err := mouse.Move(startX+(endX-startX)*eased, startY); err != nil {
			_ = mouse.Up()
			return fmt.Errorf("%w: drag: %v", ErrCaptchaUnsupported, err)
		}
	}
	if err := mouse.Up(); err != nil {
		return fmt.Errorf("%w: up: %v", ErrCaptchaUnsupported, err)
	}
	return nil
}

// waitForChallengeGone polls briefly for a success marker. It returns nil when
// the marker appears, and an error when the bounded wait elapses without one;
// the caller then re-evaluates the page and reports the challenge honestly.
func waitForChallengeGone(ctx context.Context, page playwright.Page) error {
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
		if el, err := firstVisible(ctx, page, challengeGoneSelectors); err == nil && el != nil {
			_ = el.Dispose()
			return nil
		}
	}
	return errors.New("captcha did not clear")
}

func firstVisible(ctx context.Context, page playwright.Page, selectors []string) (playwright.ElementHandle, error) {
	for _, selector := range selectors {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		el, err := page.QuerySelector(selector)
		if err != nil || el == nil {
			continue
		}
		visible, err := el.IsVisible()
		if err == nil && visible {
			return el, nil
		}
		_ = el.Dispose()
	}
	return nil, nil
}
