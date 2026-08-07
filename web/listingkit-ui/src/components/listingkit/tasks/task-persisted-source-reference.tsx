"use client";

import type { ListingKitSourceReference } from "@/lib/types/listingkit";
import { Card } from "@/components/ui/card";

function trimmed(value?: string) {
  return value?.trim() ?? "";
}

export function hasPersistedSourceReference(
  source?: ListingKitSourceReference | null,
) {
  if (!source) {
    return false;
  }

  return [source.key, source.type, source.platform, source.id, source.url].some(
    (value) => Boolean(trimmed(value)),
  );
}

export function TaskPersistedSourceReference({
  source,
}: {
  source?: ListingKitSourceReference | null;
}) {
  if (!source || !hasPersistedSourceReference(source)) {
    return null;
  }

  const platform = trimmed(source.platform);
  const id = trimmed(source.id);
  const title = [platform, id].filter(Boolean).join(" · ") ||
    trimmed(source.key) ||
    trimmed(source.type) ||
    "已记录来源";
  const sourceUrl = trimmed(source.url);

  return (
    <Card className="p-6">
      <div className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-[0.24em] text-muted-foreground">
          任务来源
        </p>
        <h2 className="text-lg font-semibold text-foreground">来源 {title}</h2>
        {source.type ? (
          <p className="text-sm leading-6 text-muted-foreground">
            来源类型：{source.type}
          </p>
        ) : null}
        {sourceUrl ? (
          <a
            className="inline-flex text-sm font-medium text-primary underline-offset-4 hover:underline"
            href={sourceUrl}
            rel="noreferrer"
            target="_blank"
          >
            查看来源
          </a>
        ) : null}
      </div>
    </Card>
  );
}
