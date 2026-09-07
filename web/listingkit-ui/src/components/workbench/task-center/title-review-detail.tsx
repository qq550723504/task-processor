"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import type { ProductTitleProposal } from "@/lib/api/product-title-review";
import { TitleApplyConfirmation, TitleComparison, TitleEditor } from "./title-review-presentation";
import styles from "./title-review.module.css";

export const titleReviewStateLabel = { pending: "待审核", accepted: "已接受，待应用", rejected: "已拒绝", applied: "已应用" };

export function TitleReviewDetail({ proposal: p }: { proposal: ProductTitleProposal }) {
  return <div className={styles.detailContent}>
    <h3>调整标准商品标题</h3><p className={p.state === "pending" ? styles.pending : styles.accepted}>{titleReviewStateLabel[p.state]}</p>
    <dl><dt>目标商品</dt><dd>{p.input.product_key}</dd><dt>基线版本</dt><dd>{p.input.base_version}</dd><dt>提案修订</dt><dd>{p.revision}</dd><dt>审核策略</dt><dd>{p.policy}</dd></dl>
    <TitleComparison before={p.before} after={p.after} originalTitle={p.original_title} />
    <section><h3>依据与来源</h3>{p.evidence.length ? p.evidence.map((item, index) => <details key={`${item.id}-${index}`} open={p.evidence.length === 1}>
      <summary>依据 {index + 1} · {item.reference_type}</summary>
      <dl><dt>证据标识</dt><dd>{item.id}</dd><dt>引用标识</dt><dd>{item.reference_id}</dd><dt>来源快照</dt><dd>{item.snapshot_id}</dd><dt>校验和</dt><dd>{item.checksum}</dd></dl>
    </details>) : <p>未提供证据引用。</p>}</section>
    <section><h3>质量与未解决问题</h3><p>以下为提案质量指标，不代表批准或模型置信度。</p>
      <dl><dt>整体质量</dt><dd>{p.quality.overall}</dd><dt>证据覆盖</dt><dd>{p.quality.evidence_coverage}</dd><dt>必填项覆盖</dt><dd>{p.quality.required_field_coverage}</dd></dl>
      {p.unresolved.length ? <ul>{p.unresolved.map((item, index) => <li key={index}>{item}</li>)}</ul> : <p>未列出未解决问题。</p>}
    </section>
    <section><h3>审核记录</h3>{p.decisions.length ? <ol>{p.decisions.map((item) => <li key={item.revision}>
      <p>{{ accept: "接受", edit: "编辑", reject: "拒绝" }[item.action]} · 修订 {item.revision} · {item.actor}</p>
      <time dateTime={item.at}>{item.at}</time><details><summary>查看本次标题变化</summary><p>修改前：{item.before}</p><p>修改后：{item.after}</p></details>
    </li>)}</ol> : <p>暂无人工审核记录。</p>}</section>
    {p.apply_receipt ? <section aria-label="应用回执"><h3>已生成新版本</h3><dl>
      <dt>新商品版本</dt><dd>{p.apply_receipt.product_version}</dd><dt>应用修订</dt><dd>{p.apply_receipt.revision}</dd>
      <dt>回执引用</dt><dd>{p.apply_receipt.publication_id}</dd><dt>操作人</dt><dd>{p.apply_receipt.actor}</dd><dt>应用时间</dt><dd><time dateTime={p.apply_receipt.at}>{p.apply_receipt.at}</time></dd>
    </dl><p>平台资料尚未同步；本次未操作平台。既有平台资料保留原商品版本。</p></section> : null}
  </div>;
}

export function TitleReviewControls({ proposal, roles, userId, pending, decide, apply }: {
  proposal: ProductTitleProposal; roles: readonly string[]; userId: string; pending: boolean;
  decide: (action: "accept" | "edit" | "reject", title?: string) => void; apply: () => void;
}) {
  const [mode, setMode] = useState<"view" | "edit" | "confirm">("view");
  // Current effective-org role hints only; every write is authorized again by Go.
  const admin = roles.includes("listingkit_admin") || roles.includes("platform_admin");
  const editor = admin || (roles.includes("listingkit_operator") && proposal.owner === userId);
  if (proposal.state === "applied" || proposal.state === "rejected") return null;
  return <section className={styles.controls} aria-label="人工审核操作"><h3>等待你确认</h3>
    <p>{admin ? "接受仅表示批准此修订，之后仍需单独应用。操作时将重新校验权限。" : "仅当前企业具备读写权限的管理员可最终接受、拒绝和应用。"}</p>
    {mode === "edit" ? <TitleEditor title={proposal.after} pending={pending} onCancel={() => setMode("view")} onSave={(title) => decide("edit", title)} /> : <div className={styles.actions}>
      {admin && proposal.state === "pending" ? <Button disabled={pending} onClick={() => decide("accept")}>接受提案</Button> : null}
      {editor ? <Button variant="outline" disabled={pending} onClick={() => setMode("edit")}>编辑标题</Button> : null}
      {admin ? <Button variant="outline" disabled={pending} onClick={() => decide("reject")}>拒绝提案</Button> : null}
      {admin && proposal.state === "accepted" ? <Button disabled={pending} onClick={() => setMode("confirm")}>应用到标准商品</Button> : null}
    </div>}
    {mode === "confirm" && admin && proposal.state === "accepted" ? <TitleApplyConfirmation productKey={proposal.input.product_key} baseVersion={proposal.input.base_version} revision={proposal.revision} pending={pending} onCancel={() => setMode("view")} onConfirm={() => { setMode("view"); apply(); }} /> : null}
  </section>;
}
