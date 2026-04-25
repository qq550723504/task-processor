"use client";

import {
  presentActionCtaKind,
} from "@/components/listingkit/shared/action-presentation";
import { derivePlatformRecoveryPresentation } from "@/components/listingkit/shared/platform-recovery";
import { Badge } from "@/components/shared/badge";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import { presentPlatformStatus } from "@/components/listingkit/shared/status-presentation";
import { cn } from "@/lib/utils/cn";
import type { PlatformCard, RecoveryDescriptor } from "@/lib/types/listingkit";

export function PlatformCardRail({
  cards,
  selectedPlatform,
  onSelect,
  onSelectRecovery,
}: {
  cards?: PlatformCard[];
  selectedPlatform?: string;
  onSelect: (card: PlatformCard) => void;
  onSelectRecovery?: (descriptor: RecoveryDescriptor, card: PlatformCard) => void;
}) {
  return (
    <div className="grid min-w-0 gap-3 [grid-template-columns:repeat(auto-fit,minmax(min(20rem,100%),1fr))]">
      {(cards ?? []).map((card) => {
        const status = presentPlatformStatus(card);
        const recovery = derivePlatformRecoveryPresentation(card);

        return (
          <Card
            key={card.platform}
            className={cn(
              "p-4 transition hover:border-zinc-400",
              selectedPlatform === card.platform && "border-zinc-950",
            )}
          >
            <div className="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0 flex-1">
                <h3 className="text-base font-semibold capitalize text-zinc-950">
                  {card.platform}
                </h3>
                {card.summary ? (
                  <p className="mt-1 break-all text-sm leading-6 text-zinc-600">
                    {card.summary}
                  </p>
                ) : null}
                {card.resolved_action_summary?.title ? (
                  <div className="mt-3 space-y-2">
                    <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                      Next step
                    </p>
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="break-words text-sm font-medium text-zinc-900">
                        {card.resolved_action_summary.title}
                      </span>
                      <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
                        {presentActionCtaKind(card.resolved_action_summary.cta_kind)}
                      </span>
                    </div>
                    <Button tone="secondary" onClick={() => onSelect(card)}>
                      {card.resolved_action_summary.title}
                    </Button>
                  </div>
                ) : null}
                {recovery ? (
                  <div className="mt-3 space-y-2">
                    <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
                      Recovery
                    </p>
                    <p className="break-words text-sm font-medium text-zinc-900">
                      {recovery.presentation.title}
                    </p>
                    <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">
                      {recovery.presentation.metaLabel}
                    </p>
                    {recovery.descriptor ? (
                      <Button
                        tone="secondary"
                        onClick={() =>
                          onSelectRecovery?.(
                            recovery.descriptor,
                            card,
                          )
                        }
                      >
                        {recovery.presentation.ctaLabel}
                      </Button>
                    ) : null}
                  </div>
                ) : null}
              </div>
              <div className="shrink-0 self-start">
                <Badge tone={status.tone}>{status.label}</Badge>
              </div>
            </div>
          </Card>
        );
      })}
    </div>
  );
}
