import { Clock3, LoaderCircle } from "lucide-react";

import { Card } from "@/components/shared/card";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

function noticeCopy(status?: string) {
  if (status === "processing") {
    return {
      title: "正在生成图片",
      description:
        "系统正在补齐预览、审核和提交所需结果。你可以留在这里等待，也可以稍后回到任务列表继续。",
      icon: LoaderCircle,
      iconClassName: "animate-spin text-sky-600",
    };
  }

  if (status === "pending") {
    return {
      title: "正在等待开始",
      description:
        "任务已经创建成功，系统正在排队准备处理。现在可以离开页面，稍后从任务列表继续。",
      icon: Clock3,
      iconClassName: "text-zinc-500",
    };
  }

  return undefined;
}

export function TaskProgressNotice({
  task,
}: {
  task?: ListingKitTaskResult | null;
}) {
  const copy = noticeCopy(task?.status);
  if (!copy) {
    return null;
  }

  const Icon = copy.icon;

  return (
    <Card className="border-sky-200 bg-sky-50/60 p-4">
      <div className="flex items-start gap-3">
        <Icon className={`mt-0.5 h-5 w-5 ${copy.iconClassName}`} />
        <div className="space-y-1">
          <p className="text-sm font-semibold text-zinc-950">{copy.title}</p>
          <p className="text-sm leading-6 text-zinc-600">{copy.description}</p>
        </div>
      </div>
    </Card>
  );
}
