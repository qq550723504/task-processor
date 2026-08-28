"use client";

import Image from "next/image";
import Link from "next/link";
import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { Phone } from "lucide-react";

import styles from "./marketing-homepage.module.css";

type SubmissionState = "idle" | "submitting" | "success" | "error";

export function ContactPanel() {
  const [open, setOpen] = useState(false);
  const [state, setState] = useState<SubmissionState>("idle");
  const [message, setMessage] = useState("");
  const launcherRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLElement>(null);
  const requestControllerRef = useRef<AbortController | null>(null);
  const submissionIdRef = useRef(0);

  const close = useCallback(() => {
    submissionIdRef.current += 1;
    requestControllerRef.current?.abort();
    requestControllerRef.current = null;
    setOpen(false);
    setState("idle");
    setMessage("");
    launcherRef.current?.focus();
  }, []);

  useEffect(() => {
    if (!open) return;
    const panel = panelRef.current;
    if (!panel) return;

    panel.querySelector<HTMLElement>("[data-autofocus]")?.focus();
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close();
        return;
      }
      if (event.key !== "Tab") return;

      const focusable = Array.from(panel.querySelectorAll<HTMLElement>("button:not([disabled]), input:not([disabled]), textarea:not([disabled]), [href]"));
      const first = focusable[0];
      const last = focusable.at(-1);
      if (!first || !last) return;
      if (!panel.contains(document.activeElement)) {
        event.preventDefault();
        first.focus();
        return;
      }
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [close, open]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const submissionId = ++submissionIdRef.current;
    const controller = new AbortController();
    requestControllerRef.current = controller;
    setState("submitting");
    setMessage("");

    try {
      const response = await fetch("/api/demo-requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        signal: controller.signal,
        body: JSON.stringify({ type: "contact", contact: data.get("contact"), message: data.get("message") }),
      });
      if (!response.ok) throw new Error("Contact request failed");
      if (submissionId !== submissionIdRef.current) return;
      form.reset();
      setState("success");
      setMessage("联系方式已提交，我们会尽快与您联系。");
    } catch {
      if (controller.signal.aborted || submissionId !== submissionIdRef.current) return;
      setState("error");
      setMessage("提交未成功，请稍后重试。");
    } finally {
      if (submissionId === submissionIdRef.current) requestControllerRef.current = null;
    }
  }

  return <>
    <button ref={launcherRef} className={styles.floatingContact} type="button" aria-haspopup="dialog" aria-expanded={open} aria-label="联系硕米" onClick={() => setOpen(true)}>
      <Phone size={18} /><span>联系</span>
    </button>
    {open ? <div className={styles.contactOverlay} onMouseDown={(event) => { if (event.target === event.currentTarget) close(); }}>
      <section ref={panelRef} className={styles.contactPanel} role="dialog" aria-modal="true" aria-labelledby="contact-panel-title">
        <button data-autofocus className={styles.contactClose} type="button" aria-label="关闭联系浮层" onClick={close}><Image src="/sumi/contact-close.svg" alt="" width={14} height={14} /></button>
        <h2 id="contact-panel-title">联系我们</h2>
        <p>微信扫码咨询，或留下您的联系方式。</p>
        <div className={styles.contactPanelBody}>
          <div className={styles.contactQr}>
            <Image src="/sumi/customer-service-qr.png" alt="微信扫码咨询客服" width={128} height={166} priority />
          </div>
          <form className={styles.contactForm} onSubmit={submit}>
            <label>电话号码<input autoComplete="tel" name="contact" placeholder="请输入您的手机号码" required type="tel" /></label>
            <label>留言内容<textarea maxLength={1000} name="message" placeholder="请简单描述您的需求…" rows={3} /></label>
            <button disabled={state === "submitting"} type="submit">{state === "submitting" ? "提交中…" : "提交联系信息"}</button>
            <small>提交即表示同意我们为本次业务沟通使用您的信息。请阅读<Link href="/privacy-policy">隐私政策</Link>和<Link href="/user-agreement">用户协议</Link>。</small>
            {message ? <p className={state === "success" ? styles.contactSuccess : styles.contactError} role="status">{message}</p> : null}
          </form>
        </div>
      </section>
    </div> : null}
  </>;
}
