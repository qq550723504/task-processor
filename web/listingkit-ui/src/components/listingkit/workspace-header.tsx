"use client";

import { RecoverySummaryCard } from "@/components/listingkit/recovery-summary-card";
import { ResolvedActionCard } from "@/components/listingkit/resolved-action-card";
import type {
  RecoveryDescriptor,
  RecoverySummary,
  ResolvedActionSummary,
} from "@/lib/types/listingkit";

export function WorkspaceHeader({
  title,
  summary,
  recoverySummary,
  onSelectAction,
  onSelectRecovery,
}: {
  title: string;
  summary?: ResolvedActionSummary | null;
  recoverySummary?: RecoverySummary | null;
  onSelectAction?: (summary: ResolvedActionSummary) => void;
  onSelectRecovery?: (descriptor: RecoveryDescriptor) => void;
}) {
  return (
    <section className="grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(20rem,1fr)]">
      <div className="space-y-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
            ListingKit Workspace
          </p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-zinc-950">
            {title}
          </h1>
        </div>
        <ResolvedActionCard summary={summary} onSelect={onSelectAction} />
      </div>
      <RecoverySummaryCard summary={recoverySummary} onSelect={onSelectRecovery} />
    </section>
  );
}
