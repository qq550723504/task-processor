"use client";

import { useEffect, useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import styles from "./title-review.module.css";

// Presentation props only. The #344 consumer supplies validated values.
export function TitleComparison({ before, after, originalTitle }: { before: string; after: string; originalTitle: string }) {
  return <section className={styles.comparison} aria-label="标题修改前后对比">
    <h3>标题修改</h3>
    <div className={styles.before}><h4>修改前</h4><p>{before}</p></div>
    <div className={styles.after}><h4>修改后</h4><p>{after}</p></div>
    {originalTitle !== after ? <details><summary>查看原始建议标题</summary><p>{originalTitle}</p></details> : null}
  </section>;
}

export function TitleEditor({ title, pending, onSave, onCancel }: {
  title: string; pending: boolean; onSave: (title: string) => void; onCancel: () => void;
}) {
  const [text, setText] = useState(title);
  const id = useId();
  const input = useRef<HTMLTextAreaElement>(null);
  useEffect(() => { input.current?.focus(); }, []);
  return <form className={styles.comparison} onSubmit={(event) => { event.preventDefault(); if (!pending) onSave(text); }}>
    <label htmlFor={id}>编辑标题</label>
    <Textarea ref={input} id={id} value={text} disabled={pending} onChange={(event) => setText(event.target.value)} aria-describedby={`${id}-hint`} />
    <p id={`${id}-hint`}>保存后旧批准失效，需重新审核接受，再单独应用。标题最多 4096 UTF-8 字节。</p>
    <div className={styles.actions}><Button type="submit" disabled={pending}>保存为待审核提案</Button><Button variant="outline" disabled={pending} onClick={onCancel}>取消编辑</Button></div>
  </form>;
}

export function TitleApplyConfirmation({ productKey, baseVersion, revision, pending, onConfirm, onCancel }: {
  productKey: string; baseVersion: string; revision: string; pending: boolean; onConfirm: () => void; onCancel: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const id = useId();
  useEffect(() => {
    const element = dialog.current;
    const previousFocus = document.activeElement;
    element?.showModal();
    cancel.current?.focus();
    return () => { element?.close(); if (previousFocus instanceof HTMLElement && previousFocus.isConnected) previousFocus.focus(); };
  }, []);
  return <dialog ref={dialog} className={styles.confirmation} aria-labelledby={`${id}-title`} aria-describedby={`${id}-description`}
    onCancel={(event) => { event.preventDefault(); if (!pending) onCancel(); }}>
    <h2 id={`${id}-title`}>应用到标准商品</h2>
    <p id={`${id}-description`}>本次仅修改标题。确认后产生新的标准商品版本；平台资料尚未同步，本次未操作平台。</p>
    <dl><dt>目标商品</dt><dd>{productKey}</dd><dt>基线版本</dt><dd>{baseVersion}</dd><dt>已接受提案修订</dt><dd>{revision}</dd></dl>
    <div className={styles.actions}>
      <Button ref={cancel} variant="outline" disabled={pending} onClick={onCancel}>返回审核</Button>
      <Button disabled={pending} onClick={onConfirm}>确认应用</Button>
    </div>
    {pending ? <p role="status">正在提交，请等待结果。停止等待不代表撤销服务器提交。</p> : null}
  </dialog>;
}
