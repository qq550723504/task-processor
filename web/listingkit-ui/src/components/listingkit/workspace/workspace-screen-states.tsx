import Link from "next/link";
import { LoaderCircle } from "lucide-react";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/shared/empty-state";

type RetryHandler = () => Promise<unknown> | void;

export function WorkspaceLoadingState() {
  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <LoaderCircle className="h-8 w-8 animate-spin text-zinc-400" />
    </div>
  );
}

export function WorkspaceLoadErrorState({ onRetry }: { onRetry: RetryHandler }) {
  return (
    <EmptyState
      title="工作台暂时无法加载"
      description="当前无法完整读取任务状态、预览或审核会话。你可以刷新重试，或先回到任务列表重新进入。"
      action={<WorkspaceRetryActions label="刷新当前页面" onRetry={onRetry} />}
    />
  );
}

export function WorkspacePendingDataState({ onRetry }: { onRetry: RetryHandler }) {
  return (
    <EmptyState
      title="工作台数据暂未准备完成"
      description="当前任务还没有返回完整的预览和审核会话数据。可以稍后刷新，或先回到任务列表查看任务状态。"
      action={<WorkspaceRetryActions label="重新加载" onRetry={onRetry} />}
    />
  );
}

function WorkspaceRetryActions({
  label,
  onRetry,
}: {
  label: string;
  onRetry: RetryHandler;
}) {
  return (
    <div className="flex flex-wrap gap-3">
      <Button
        onClick={() => void onRetry()}
        type="button"
      >
        {label}
      </Button>
      <Button asChild variant="outline">
        <Link href="/listing-kits">返回任务列表</Link>
      </Button>
    </div>
  );
}
