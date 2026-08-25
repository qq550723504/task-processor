# Temporary preflight input simplification

## Red/green evidence

- RED: `go test ./cmd/listingkit-phone-onboarding-preflight -run TestReadSecretReadsSuccessiveLinesFromPowerShellLikeStdin -count=1` failed with `secure input failed` while the existing `term.ReadPassword`/console-input path handled a regular PowerShell-like stdin file.
- GREEN: After replacing the terminal-specific path with a plain line reader, the same focused test passed.

The regression test supplies two newline-terminated lines through one `*os.File` and calls `readSecret` twice, proving that the second line is preserved.

## Changed files

- Simplified `readSecret` to read ordinary terminal lines directly, retaining prompt output, trimming, generic input errors, and existing context/abandon cleanup behavior.
- Added the successive-line input regression test.
- Removed Windows/non-Windows secret-input helper implementations and tests.
- Removed the now-unused root-module `golang.org/x/term` requirement and checksums.

No ZITADEL request, client, runner, token collection, or stable redacted output behavior was changed.

## Verification

- `go test ./cmd/listingkit-phone-onboarding-preflight -count=1` — PASS
- `go test ./cmd/listingkit-phone-onboarding-preflight ./internal/listingkit/phoneonboardingpreflight -count=1` — PASS
- `go test ./internal/listingkit/zitadelsms ./internal/listingkit/memberinvite -count=1` — PASS
- `go test -race ./cmd/listingkit-phone-onboarding-preflight ./internal/listingkit/phoneonboardingpreflight -count=1` — PASS

Plain input is accepted only for this temporary preflight CLI; it is not a general credential-input policy.

## Review fix 1: dependency checksum evidence

- `go mod tidy -diff` showed no direct `golang.org/x/term` requirement, while proposing the seven `x/term` checksum entries restored here.
- `go list -m all` before restoration reported the missing `golang.org/x/term v0.17.0/go.mod` checksum through the transitive `golang.org/x/crypto` dependency.
- Restored only the seven root `go.sum` entries; `go.mod` remains without a direct `x/term` requirement.
