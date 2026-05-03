"use client";

import type { TaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";
import { Card } from "@/components/shared/card";

function summarizeSource(draft?: Partial<TaskCreateDraft> | null) {
  const hasProductUrl = Boolean(draft?.productUrl?.trim());
  const imageCount =
    draft?.imageUrls
      ?.split(/\r?\n/)
      .map((value) => value.trim())
      .filter(Boolean).length ?? 0;

  if (hasProductUrl && imageCount > 0) {
    return {
      title: "来自商品链接和图片素材",
      summary: `本次任务同时提交了商品链接和 ${imageCount} 张图片，用于补充来源信息和生成素材。`,
    };
  }

  if (hasProductUrl) {
    return {
      title: "来自商品链接",
      summary: "这个任务从商品页链接开始创建，适合已经有明确原始商品来源的场景。",
    };
  }

  if (imageCount > 0) {
    return {
      title: "来自图片素材",
      summary: `这个任务提交了 ${imageCount} 张图片，系统会根据图片内容继续生成和审核。`,
    };
  }

  return null;
}

export function TaskSourceSummary({
  draft,
}: {
  draft?: Partial<TaskCreateDraft> | null;
}) {
  const source = summarizeSource(draft);
  if (!source) {
    return null;
  }

  return (
    <Card className="p-6">
      <div className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
          任务来源
        </p>
        <h2 className="text-lg font-semibold text-zinc-950">{source.title}</h2>
        <p className="text-sm leading-6 text-zinc-600">{source.summary}</p>
      </div>
    </Card>
  );
}

