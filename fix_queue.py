import io, os, re, sys

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
p = "internal/integration/acquisition/a1688/browser/throttle.go"
s = open(p, encoding="utf-8").read()

# --- state: an ordered reservation list instead of ad-hoc rollback bookkeeping
old_state = """	mu          sync.Mutex
	next        time.Time // earliest allowed start
	cooledAt    time.Time
	blocked     bool
	seq         uint64        // identifies the newest reservation, for safe rollback
	prevNext    time.Time     // boundary the newest reservation displaced
	superseded  time.Time     // boundary of a cancelled reservation that was superseded"""
if old_state not in s:
    m = re.search(r"\tmu\s+sync\.Mutex[\s\S]*?superseded\s+time\.Time[^\n]*\n", s)
    assert m, "state block not found"
    old_state = m.group(0)
new_state = """	mu       sync.Mutex
	next     time.Time // earliest allowed start
	cooledAt time.Time
	blocked  bool
	// pending holds live reservations in start order. Keeping them explicitly
	// makes releasing one correct regardless of release order: next is
	// recomputed from what is left, instead of being reconstructed by trying to
	// undo an earlier boundary.
	pending []reservation"""
s = s.replace(old_state, new_state, 1)

# --- reservation record
s = s.replace("type Throttle struct {", """// reservation is one committed slot: the caller may start at start, and the next
// caller must not start before end.
type reservation struct {
	seq   uint64
	start time.Time
	end   time.Time
}

type Throttle struct {""", 1)

# --- acquire: append and take the next end as the new floor
old_acq = """	t.prevNext = t.next
	t.seq++
	mine := t.seq
	t.next = slotEnd
	t.mu.Unlock()"""
new_acq = """	t.seq++
	mine := t.seq
	t.pending = append(t.pending, reservation{seq: mine, start: start, end: slotEnd})
	t.next = slotEnd
	t.mu.Unlock()"""
assert old_acq in s, "acquire anchor"
s = s.replace(old_acq, new_acq, 1)

# --- release: drop the entry and recompute
old_rel_start = s.index("// release gives back a reservation")
old_rel_end = s.index("\n}\n", s.index("func (t *Throttle) release", old_rel_start)) + 3
new_rel = """// release gives back a reservation the caller could not use. next is recomputed
// from the reservations that remain, so releasing is correct whether or not
// other callers reserved in the meantime.
func (t *Throttle) release(seq uint64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	kept := t.pending[:0]
	for _, r := range t.pending {
		if r.seq != seq {
			kept = append(kept, r)
		}
	}
	t.pending = kept
	t.next = time.Time{}
	for _, r := range kept {
		if r.end.After(t.next) {
			t.next = r.end
		}
	}
}
"""
s = s[:old_rel_start] + new_rel + s[old_rel_end:]

# --- seq counter must exist
s = s.replace("	// rand is guarded by mu.\n	rand *rand.Rand",
              "	// seq identifies reservations; it is guarded by mu.\n\tseq uint64\n\t// rand is guarded by mu.\n\trand *rand.Rand", 1)

open(p, "w", encoding="utf-8").write(s)
print("release recomputes the floor from live reservations")
