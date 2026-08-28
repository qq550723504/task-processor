import { CheckCircle2, LoaderCircle, ShieldAlert, Sparkles } from "lucide-react";

import type { ProductWorkspaceAttentionItem } from "@/components/listingkit/workspace/product-workspace-model";
import type { ProductWorkspaceReviewIssue } from "@/components/listingkit/workspace/product-workspace-review-model";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export function ProductWorkspaceAIReview({
  summary,
  issues,
  onSelectIssue,
  checking = false,
}: {
  summary: readonly ProductWorkspaceAttentionItem[];
  issues: readonly ProductWorkspaceReviewIssue[];
  onSelectIssue: (issue: ProductWorkspaceReviewIssue) => void;
  checking?: boolean;
}) {
  return (
    <Card className="min-w-0 border-border bg-card p-4">
      <div className="flex items-center gap-2">
        <Sparkles className="h-4 w-4 text-teal-700" />
        <h2 className="text-sm font-semibold text-foreground">AI 审核</h2>
      </div>

      <div className="mt-4 grid grid-cols-3 gap-2">
        {summary.map((item) => (
          <div className="rounded-lg border border-border bg-muted/40 p-2" key={item.severity}>
            <div className="text-lg font-semibold text-foreground">{item.count}</div>
            <div className="mt-0.5 text-[11px] leading-4 text-muted-foreground">{item.label}</div>
          </div>
        ))}
      </div>

      {issues.length === 0 ? (
        checking ? (
          <div className="mt-4 rounded-lg border border-border bg-muted/30 p-4">
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <LoaderCircle className="h-4 w-4 animate-spin text-teal-700" />
              AI 正在检查商品
            </div>
            <p className="mt-2 text-xs leading-5 text-muted-foreground">
              检查完成后，会在这里列出需要你处理的问题。
            </p>
          </div>
        ) : (
          <div className="mt-4 rounded-lg border border-border bg-muted/30 p-4">
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <CheckCircle2 className="h-4 w-4 text-emerald-700" />
              没有需要你处理的问题
            </div>
            <p className="mt-2 text-xs leading-5 text-muted-foreground">
              AI 检查已完成，可以继续当前操作。
            </p>
          </div>
        )
      ) : (
        <div className="mt-4 space-y-3">
          {issues.map((issue) => (
            <IssueCard issue={issue} key={issue.id} onSelect={onSelectIssue} />
          ))}
        </div>
      )}
    </Card>
  );
}

function IssueCard({
  issue,
  onSelect,
}: {
  issue: ProductWorkspaceReviewIssue;
  onSelect: (issue: ProductWorkspaceReviewIssue) => void;
}) {
  const severityLabel = issue.severity === "blocking" ? "必须处理" : "建议确认";

  return (
    <div className="rounded-lg border border-border bg-background p-3">
      <div className="flex items-start gap-2">
        <ShieldAlert
          aria-hidden="true"
          className={
            issue.severity === "blocking"
              ? "mt-0.5 h-4 w-4 shrink-0 text-destructive"
              : "mt-0.5 h-4 w-4 shrink-0 text-amber-700"
          }
        />
        <div className="min-w-0 flex-1">
          <div className="text-[11px] font-medium leading-4 text-muted-foreground">
            {severityLabel}
          </div>
          <h3 className="mt-1 text-sm font-semibold text-foreground">{issue.title}</h3>
          {issue.description ? (
            <p className="mt-1 text-xs leading-5 text-muted-foreground">{issue.description}</p>
          ) : null}
          {issue.suggestion ? (
            <p className="mt-2 text-xs leading-5 text-foreground">AI 建议：{issue.suggestion}</p>
          ) : null}
          {issue.evidence ? (
            <p className="mt-1 text-xs leading-5 text-muted-foreground">依据：{issue.evidence}</p>
          ) : null}
          {typeof issue.confidence === "number" ? (
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              置信度：{Math.round(issue.confidence * 100)}%
            </p>
          ) : null}
        </div>
      </div>
      {issue.actionKey ? (
        <Button
          aria-label={`处理：${issue.title}`}
          className="mt-3 w-full"
          onClick={() => onSelect(issue)}
          size="sm"
          type="button"
          variant="secondary"
        >
          处理
        </Button>
      ) : null}
    </div>
  );
}
