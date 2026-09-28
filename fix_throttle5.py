import io, os, sys

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
p = "internal/integration/acquisition/a1688/browser/throttle.go"
s = open(p, encoding="utf-8").read()

# --- (3) quarantine must derive from the CONFIGURED cooldown, not a constant ---
old = """	// Zero (the unset default) means "use the conservative default"; a negative
	// value explicitly disables the quarantine, which only tests need. Disabling
	// it by default would be the unsafe direction.
	quarantined := startupQuarantine >= 0
	if startupQuarantine == 0 {
		startupQuarantine = DefaultStartupQuarantine
	}"""
new = """	// Zero (the unset default) means "follow the configured cooldown"; a negative
	// value explicitly disables the quarantine, which only tests need. Disabling
	// it by default would be the unsafe direction.
	//
	// It follows the CONFIGURED cooldown, not the packaged constant: an operator
	// who deliberately shortens -challenge-cooldown must not still get a
	// ten-minute silence on every restart, or the setting is not actually
	// tunable.
	quarantined := startupQuarantine >= 0
	if startupQuarantine == 0 {
		startupQuarantine = challengeCooldown
	}"""
assert old in s, "quarantine derivation anchor"
s = s.replace(old, new, 1)

s = s.replace("""	// DefaultStartupQuarantine matches the cooldown by default: a replacement
	// collector must assume the worst about an exit IP it did not observe.
	DefaultStartupQuarantine = DefaultChallengeCooldown
""", "")
s = s.replace("""	// DefaultStartupQuarantine matches the cooldown by default: a replacement
	// collector must assume the worst about an exit IP it did not observe.
	DefaultStartupQuarantine = DefaultChallengeCooldown
)""", ")")
s = s.replace("""	DefaultChallengeCooldown = 10 * time.Minute
)""", """	// A replacement collector must assume the worst about an exit IP it never
	// observed, so the startup quarantine follows the configured cooldown.
	DefaultChallengeCooldown = 10 * time.Minute
)""")

# --- (2) re-check the cooldown after a queued wait, and (4) drop the slot -----
old_wait_tail = """	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		t.release(mine)
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}"""
new_wait_tail = """	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		t.release(mine)
		return ctx.Err()
	case <-timer.C:
	}
	// The wait may have spanned another acquisition, and that one may have
	// observed a challenge. Returning success here would hand the caller a slot
	// the process has already decided to refuse, so the block is re-checked once
	// the wait is over and the reservation is given back if the process is now
	// cooling down.
	if t.nowBlocked() {
		t.release(mine)
		return ErrThrottled
	}
	return nil
}

// nowBlocked reports whether a challenge has put the process into cooldown,
// after lazily expiring a window that has already elapsed.
func (t *Throttle) nowBlocked() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.blocked && !t.cooledAt.IsZero() && !time.Now().Before(t.cooledAt) {
		t.blocked = false
		t.cooledAt = time.Time{}
	}
	return t.blocked
}"""
assert old_wait_tail in s, "wait tail anchor"
s = s.replace(old_wait_tail, new_wait_tail, 1)

# --- (4) a cancelled reservation must leave the queue even if superseded ------
old_release = """	t.mu.Lock()
	defer t.mu.Unlock()
	if t.seq == seq {
		// Restore the boundary this reservation displaced. Clearing the schedule
		// outright would let the next caller start immediately after the preceding
		// acquisition and recreate the burst this throttle exists to prevent.
		t.next = t.prevNext
	}"""
new_release = """	t.mu.Lock()
	defer t.mu.Unlock()
	// If a later caller reserved after us, we are not the newest reservation, so
	// restoring prevNext would discard that caller's slot. Instead remove our own
	// contribution: our slot is [start, prevNext) and the newest reservation begins
	// at prevNext, so pulling `next` back to prevNext keeps the later caller's
	// spacing and drops ours.
	if t.seq == seq {
		// Restore the boundary this reservation displaced. Clearing the schedule
		// outright would let the next caller start immediately after the preceding
		// acquisition and recreate the burst this throttle exists to prevent.
		t.next = t.prevNext
		return
	}
	// We were superseded. The only value we may safely remove is the front of the
	// queue we occupied, and only if the newest reservation still starts where we
	// left it; otherwise a later caller legitimately depends on our spacing and
	// dropping it would hand two callers the same slot.
	if t.next.After(t.prevNext) && t.prevNext.After(time.Time{}) {
		if t.superseded == 0 {
			t.superseded = t.prevNext
		}
		if !t.superseded.After(t.prevNext) {
			t.next = t.prevNext
		}
	}"""
assert old_release in s, "release anchor"
s = s.replace(old_release, new_release, 1)

s = s.replace("""	prevNext time.Time // boundary the newest reservation displaced""",
              """	prevNext time.Time // boundary the newest reservation displaced
	superseded time.Time // boundary of a cancelled reservation that was superseded""")

open(p, "w", encoding="utf-8").write(s)
print("throttle: quarantine follows the configured cooldown; cooldown rechecked after wait; superseded release fixed")
