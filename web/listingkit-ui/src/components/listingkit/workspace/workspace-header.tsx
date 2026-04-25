"use client";

import { RecoverySummaryCard } from "@/components/listingkit/review/recovery-summary-card";
import { ResolvedActionCard } from "@/components/listingkit/review/resolved-action-card";
import type {
  RecoveryDescriptor,
  RecoverySummary,
  ResolvedActionSummary,
} from "@/lib/types/listingkit";

export function WorkspaceHeader({
  title,
  subtitle,
  summary,
  recoverySummary,
  onSelectAction,
  onSelectRecovery,
}: {
  title: string;
  subtitle?: string;
  summary?: ResolvedActionSummary | null;
  recoverySummary?: RecoverySummary | null;
  onSelectAction?: (summary: ResolvedActionSummary) => void;
  onSelectRecovery?: (descriptor: RecoveryDescriptor) => void;
}) {
  return (
    <section className="grid min-w-0 gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(20rem,1fr)]">
      <div className="min-w-0 space-y-3">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
            ListingKit Workspace
          </p>
          <h1 className="mt-2 break-words text-2xl font-semibold tracking-tight text-zinc-950 sm:text-3xl">
            {title}
          </h1>
          {subtitle ? (
            <p className="mt-2 break-all font-mono text-xs text-zinc-500">
              {subtitle}
            </p>
          ) : null}
        </div>
        <ResolvedActionCard summary={summary} onSelect={onSelectAction} />
      </div>
      <RecoverySummaryCard summary={recoverySummary} onSelect={onSelectRecovery} />
    </section>
  );
}
