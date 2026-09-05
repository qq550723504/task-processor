# Source-account ownership preflight (slice A)

This is a read-only preview, not a backfill or a production cutover gate. The full
inventory, target contract and A–D rollout dependencies are in
[the design](../superpowers/specs/2026-09-05-source-account-organization-cutover.md).

Run on the host with the actual 1688 account profile volume mounted at its runtime
path. Use the same absolute profile root that currently feeds
`NewAccountProfileResolver`; do not substitute the public/anonymous browser directory.
The stored ProfileRef is currently only a marker; the actual directory is
`<root>/<legacy tenant>/<account ID>`. Existing directories are verified and reused
as opaque references in the receipt. Nothing is created, moved, deleted or launched.
Missing profiles (including disabled/deleted accounts) block; resolve their retention
and runtime history explicitly rather than creating empty replacement profiles. The
preflight derives an indexable filesystem identity for each account profile and rejects
duplicates, so two account paths that point to the same underlying browser profile (for
example through a bind mount) fail closed without pairwise scanning across all accounts.

Linux also checks each account's backing subtree and nested mounts for overlap with
other accounts, even when the account-root inodes differ. Mountpoints and account
ancestors are indexed; backing ranges are sorted by device/path and checked in one
pass. This retains near-linear/log-linear work rather than account-pair comparisons,
and does not read browser file contents. Linux additionally walks directory entries
below each account exactly once and rejects every descendant symbolic link without
following it. Relative, absolute, broken, looping and deeply nested links all block;
entry/metadata errors and cancellation block as well. Repeated backing ranges belonging
to the same account are allowed; overlap between distinct accounts blocks even within
one Organization. Regular files with multiple links are indexed by device/inode only
when their link count exceeds one; the same identity may repeat inside one account but
blocks if it appears under another account.

Windows extends the existing no-symlink/alias rule to descendants: it walks directory
entry metadata under each verified account and rejects junctions, symlinks and directory
reparse mount points before following them. Native DIRECTORY/REPARSE_POINT attributes
are checked because a junction can appear as `ModeIrregular` with `IsDir=false` in Go
directory enumeration. Ordinary directories and files remain supported; file contents
are never opened. Entry/attribute inspection failures block, and cancellation is checked
for each visited entry. Cost depends on the directory-entry inventory, not account pairs.
Regular files with multiple links receive the equivalent Windows volume/file identity
check, rejecting cross-account hard links while retaining same-account links.
Linux and Windows are the supported preflight runtime hosts. Other ports remain
compileable but fail closed before producing a digest because equivalent descendant
alias validation is not implemented.

Provide these environment variables through the existing secret mechanism:

- `SOURCE_ACCOUNT_PREFLIGHT_DSN`: business PostgreSQL database with `source_account`.
- `SOURCE_ACCOUNT_METADATA_DSN`: the explicitly selected authoritative ZITADEL
  database with `projections.org_metadata2`; no automatic candidate database discovery.

Use credentials restricted to SELECT. The command additionally uses read-only,
repeatable-read transactions. Two separate database snapshots are **not atomic**.
Verify the correct ZITADEL instance/environment before running. Projection freshness,
current authority and multi-host profile agreement must be revalidated under the B
freeze; a successful A receipt does not certify them.

```powershell
go run ./cmd/source-account-ownership-preflight `
  -source-id production/business-cluster `
  -metadata-id production/zitadel-cluster `
  -profile-root C:/runtime/1688-account-profiles `
  -receipt C:/migration-evidence/1688-ownership-preflight-001.json
```

Source identities are non-secret audit labels, not DSNs. The maintained operational script is
`scripts/source-account-ownership-preflight.ps1`, with `-ProfileRoot`, `-SourceId`,
`-MetadataId` and `-ReceiptPath` parameters. It forwards the same environment-based
connections and read-only command, preserves the caller's receipt path and propagates
failure. Direct `go run` invocation above is equivalent. The receipt path must be
absolute, its directory must already exist, and it must be outside the browser profile
root. Symlink/junction aliases in the receipt-parent path are rejected, and the receipt
parent ancestry is compared with the indexed profile root/tenant/account filesystem
identities so direct profile aliases cannot become evidence destinations. Use a new
output name for each attempt. Default total deadline is two minutes (maximum ten); each
collection is capped at 100,000 rows. Exceeding the bound fails closed rather than
truncating. A larger installation needs a separately reviewed paged snapshot
implementation, not an unbounded flag. Command errors suppress database connection
details; troubleshoot connection/schema/permissions with existing operator tools.

On Linux the mount table is read once and the receipt backing location is checked
against both the profile root and every mount nested below it, including different
devices. Empty, malformed, uncovered or ambiguous mount relationships fail closed.
This inspects mount metadata and directory identities, not profile contents, and
does not scan account pairs. Keep the mounted namespace stable during this read-only
observation; the A receipt remains non-atomic evidence, not B fleet validation.

On success, the command prints account count and SHA-256. The JSON includes each
account's old ownership/status/deleted/profile reference, mapped Organization and exact
verified profile directory, plus metadata sequences/removal flags and source observation
times. It contains no profile contents or credentials. Treat it as internal audit data.
The digest binds the captured fields and source identities/database names; observation
times are excluded so identical captured inputs give identical digests across reruns.
It does not fingerprint uncaptured account fields or browser cookie contents, and is
not a signature or authorization to mutate data. B must take its own complete before-image.

The final receipt is published exclusively from a synced temporary file using a hard
link. An existing final file is never overwritten; unsupported filesystems fail closed.
`WriteReceipt` owns the publication success decision: it observes cancellation before
the final hard link and immediately after it; if cancellation is observed after linking
but before success is committed, it removes that final link and returns an error. A nil
return commits this evidence publication; cancellation arriving after that boundary does
not retroactively turn the completed publication into a failed invocation. This avoids a
caller-side race between a successful receipt publication and a later context check. A
process interruption earlier can leave a `.ownership-receipt-*.pending` file. Such files
are not receipts. Rerun with a new output path; no database rollback or profile recovery
is needed. Power-loss durability of the directory entry depends on the filesystem; a
missing final file is handled by rerunning the read-only command.

Validation blocks on missing, removed, duplicate or ambiguous ownership; invalid
noncanonical metadata; invalid/duplicate account identity; absent/non-directory or
symlink/junction profile path (including ancestors); or two accounts sharing one
filesystem profile identity. Duplicate Organization metadata is rejected rather than
choosing a sequence and accidentally resurrecting a removed owner. The command reads all
1688 account rows, including disabled/deleted, and does not activate them.

Metadata format validation applies to every row, including `owner_removed=true`.
A valid removed row remains in the receipt but never contributes an active mapping;
an empty, nonnumeric, nonpositive, overflowing or noncanonical numeric value blocks.

### R1-A regression matrix

All filesystem tests below use disposable directories; no real browser profile is
read, moved, deleted or recreated. Fixture coverage and live platform checks are
separate evidence:

| Receipt target / evidence | Expected | Test |
| --- | --- | --- |
| Root, account, descendant | Reject | `TestReceiptTargetRejectsProfileTreeAndAlias`, `TestReceiptMountMatrix` |
| External directory, similar non-child prefix | Accept | Same tests |
| Symlink (Linux), junction (Windows) | Reject | `TestReceiptTargetRejectsProfileTreeAndAlias` |
| External bind of account / descendant | Reject | `TestReceiptMountMatrix`, `TestReceiptTargetRejectsLiveBindMountedProfileDescendant` |
| Different-device nested mount externally bound, including child path | Reject | `TestReceiptMountMatrix`, `TestReceiptTargetRejectsLiveNestedMount` |
| Unrelated mounted device | Accept | `TestReceiptMountMatrix` |
| Empty/malformed/uncovered/ambiguous mount relationship | Reject | `TestReceiptMountMatrix` |
| Unrelated stacked mounts or nsfs file mounts; repeated identical backing location | Accept external target | `TestReceiptMountMatrix` |
| Distinct account-root identities with overlapping backing subtrees or nested mounts | Reject | `TestPreflightRejectsLiveOverlappingProfileSubtrees`, `TestProfileSubtreeMountMatrix` |
| Linux relative/absolute, root/deep, broken/looping descendant symlink | Reject without receipt or mutation | `TestPreflightRejectsLinuxDescendantSymlinksWithoutPublicationOrMutation`, `TestPreflightRejectsBrokenAndLoopingLinuxDescendantSymlinks` |
| Ordinary Linux descendant directories/files; traversal cancellation/read error | Accept; cancel/reject | `TestLinuxProfileEntryWalkAcceptsOrdinaryEntries`, `TestLinuxProfileEntryWalkHonorsCancellationDuringTraversal`, `TestLinuxProfileEntryWalkFailsClosedOnReadError` |
| Linux/Windows cross-account hard-linked profile file; same-account hard link | Reject; accept | `TestPreflightRejectsCrossAccountLinuxHardLinks`, `TestPreflightAcceptsSameAccountLinuxHardLinks`, `TestPreflightRejectsCrossAccountWindowsHardLinks`, `TestPreflightAcceptsSameAccountWindowsHardLinks` |
| Windows account descendants junctioned to a shared directory | Reject | `TestPreflightRejectsDescendantJunctionOverlap` (real disposable junctions) |
| Ordinary Windows descendant directories and cancellation | Accept; cancel | `TestPreflightAcceptsOrdinaryWindowsDescendants` |
| 100,000 account roots plus 100,000 disjoint nested mounts | Accept | `TestProfileSubtreeIndexAtInventoryLimit` (fixture, not fleet acceptance) |
| Malformed removed metadata; valid tombstone with active owner; removed-only owner | Reject; retain without mapping; reject missing active owner | `TestPreflightRemovedMetadataValidation` |

R1-A TDD reproduced eight malformed removed values and both nested-device fixture
cases on the handoff implementation before applying the fixes. Windows junction and
Linux live mount checks passed locally; Linux ran in a disposable Docker container
with `SYS_ADMIN`, using a read-only test-binary mount and no production volumes.
Cross-platform builds are compile evidence only. Existing publication exclusivity,
pre-link cancellation, success boundary, deterministic digest and disabled/deleted
tests remain. `TestReceiptCancellationAfterLinkCleansFinalAndStaging` additionally
exercises cancellation at the publisher's post-link check, without a second CLI
decision. This evidence does not change #30's production cutover BLOCKER.

The first R1-A CI run exposed overly broad rejection of otherwise valid mount tables.
Follow-up TDD reproduced unrelated stacked mounts, opaque nsfs roots and repeated
identical backing locations being rejected. The resolver now checks ambiguity at
the selected root/receipt location, while retaining conservative checks of protected
nested mounts; unrelated mount entries do not invalidate a safe external directory.
Different possible backing locations at a selected mountpoint still fail closed.

A later R1-A review identified account-subtree overlap beyond equal root identities.
All three disposable Linux live cases first produced receipts on the prior head:
account-root-to-descendant bind, descendant-to-descendant bind, and two accounts
mounting the same external subtree. They now reject without usable receipts.
Disjoint devices, similar non-child prefixes, same-account nested ranges and unrelated
opaque mounts remain accepted. These checks retain the local mount-alias protections without
changing receipt publication ownership or B/C/D scope.

R2-A (finding 3940190916) explicitly chooses rejection when related storage cannot
prove isolation. Different client device numbers do not establish independent backing
storage for network exports, and equal or different source strings are not proof.
Linux accepts device-relative subtree checks on ext2/ext3/ext4, XFS, Btrfs, tmpfs and
ramfs. Selected profile/receipt mounts and protected nested mounts on other storage
fail closed, including NFS/CIFS, userspace filesystems, layered overlay and unknown
types. Overlay's backing layers are not resolved by this bounded preflight; a container
must expose supported profile and receipt volumes rather than use its overlay root.
Unrelated unsupported mounts do not reject otherwise valid local paths. Windows
rejects remote/mapped or unknown volumes before using local volume/file identities;
existing local directory and junction checks remain. Neither platform enumerates
network exports or treats source names as authoritative ownership identities.

`TestUnprovableMountStorage` first failed on e4a3d57d8 and now covers selected root,
account, descendant and receipt locations, independent devices, equal/different
source names, and unrelated network mounts. This is mountinfo fixture evidence.
`TestWindowsStorageIsolation` checks native API result policy plus a real disposable
local directory, not a live network share. Linux bind/tmpfs tests use a disposable
container with `/tmp` mounted as tmpfs; Windows junction tests use disposable NTFS
directories. Real NFS/CIFS/mapped-share acceptance remains **NOT_RUN**. This is an
explicit fail-closed support boundary, not acceptance of shared-storage risk or new
network filesystem support. Receipt publication and the #30 BLOCKER are unchanged.

R3-A (finding 3940268783) closes the Linux descendant-symlink gap without replacing
the mount checks. Its formal test first published a usable disposable receipt on
`cdf2925ff` for both relative and absolute links at root and deeper paths. The fixed
preflight rejects before publication and leaves the link target, link metadata and
fixture content unchanged. Broken and cyclic links are rejected without resolution.
The production walk uses directory entries and metadata only; it never opens profile
files for content. Each account directory is walked once, so work is O(total entries),
not account pairs or repeated scans of the shared profile root. The existing command
deadline is checked at every visited entry. `filepath.WalkDir` retains the current
traversal state plus the entries for one directory. A real tmpfs check visited 4,096
ordinary files once; this is traversal evidence, not a 100,000-profile benchmark. An
unprivileged disposable Linux run confirmed unreadable descendants fail closed.

The final incremental review also identified cross-account hard-linked profile files.
Formal Windows NTFS and Linux tmpfs tests first returned usable evidence on `a84b6b70f`.
The follow-up indexes only regular entries whose native link count exceeds one, keeping
the usual case out of the identity map. Work remains O(total entries), with memory
proportional to multiply-linked candidates rather than all profile files. Same-account
hard links remain valid, while a repeated identity under another account blocks.
Tests use `supportedTestTempDir`: Linux selects an actually supported `/dev/shm` or
system temporary mount through the same mount resolver; other platforms retain their
native temporary directory. This keeps ordinary tests repeatable in overlay-backed
containers without bypassing production validation. Dedicated live mount tests still
exercise the real filesystem path and production checks.

The final review confirmed that buildability must not imply runtime support. Darwin and
other non-Linux/non-Windows builds now return an explicit unsupported-platform error
from subtree validation instead of approving unchecked profiles. Cross-compilation is
still compile evidence only; runtime validation on those ports remains **NOT_RUN**.

The subsequent Windows review reproduced the same gap with two distinct account roots
containing junctions to a shared directory. That real junction test failed before the
Windows metadata walk and passes after it. This extends Windows alias verification
below account roots without disabling the platform or reading profile file contents.

Before B/C: freeze all 1688 admissions and source-account/mapping writers, drain and
attest every old process queue/worker, reconcile Redis and local terminal outcomes,
export/checksum retained results and verify authoritative metadata again. Redis emptiness
or a six-hour wait is not evidence that old jobs never existed. Unknown ownership or
unknown in-flight outcome blocks migration. A does not connect to Redis or stop workers.
C also needs coordination with the old #30 handoff consumer; never add a numeric adapter
to make that reader survive. Neither #30 nor #301 is closed by this prerequisite PR.

Local validation:

```powershell
go test ./internal/integration/persistence/sourceaccount/ownershipmigration -count=1
go vet ./internal/integration/persistence/sourceaccount/ownershipmigration ./cmd/source-account-ownership-preflight
go test ./internal/sourceaccount/... ./internal/crawler/alibaba1688/... ./internal/crawler/shared/... -count=1
go test ./tests -count=1
```
