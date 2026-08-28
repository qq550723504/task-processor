"use client";

import Image from "next/image";
import { FormEvent, useState } from "react";
import { Phone } from "lucide-react";

import styles from "./marketing-homepage.module.css";

type SubmissionState = "idle" | "submitting" | "success" | "error";

export function ContactPanel() {
  const [open, setOpen] = useState(false);
  const [state, setState] = useState<SubmissionState>("idle");
  const [message, setMessage] = useState("");

  function close() {
    setOpen(false);
    setState("idle");
    setMessage("");
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    setState("submitting");
    setMessage("");

    try {
      const response = await fetch("/api/demo-requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ type: "contact", contact: data.get("contact"), message: data.get("message") }),
      });
      if (!response.ok) throw new Error("Contact request failed");
      form.reset();
      setState("success");
      setMessage("联系方式已提交，我们会尽快与您联系。");
    } catch {
      setState("error");
      setMessage("提交未成功，请稍后重试。");
    }
  }

  return <>
    <button className={styles.floatingContact} type="button" aria-haspopup="dialog" aria-expanded={open} aria-label="联系硕米" onClick={() => setOpen(true)}>
      <Phone size={18} /><span>联系</span>
    </button>
    {open ? <div className={styles.contactOverlay} onMouseDown={(event) => { if (event.target === event.currentTarget) close(); }}>
      <section className={styles.contactPanel} role="dialog" aria-modal="true" aria-labelledby="contact-panel-title" onKeyDown={(event) => { if (event.key === "Escape") close(); }}>
        <button className={styles.contactClose} type="button" aria-label="关闭联系浮层" onClick={close}><Image src="/sumi/contact-close.svg" alt="" width={14} height={14} /></button>
        <h2 id="contact-panel-title">联系我们</h2>
        <p>扫码咨询，或留下您的联系方式</p>
        <div className={styles.contactPanelBody}>
          <div className={styles.contactQr} aria-label="联系二维码待配置">
            <Image src="/sumi/contact-qr-placeholder.svg" alt="" width={112} height={112} />
            <span>联系二维码待配置</span>
          </div>
          <form className={styles.contactForm} onSubmit={submit}>
            <label>电话号码<input autoComplete="tel" name="contact" placeholder="请输入您的手机号码" required type="tel" /></label>
            <label>留言内容<textarea maxLength={1000} name="message" placeholder="请简单描述您的需求…" rows={3} /></label>
            <button disabled={state === "submitting"} type="submit">{state === "submitting" ? "提交中…" : "提交联系信息"}</button>
            <small>提交即表示同意我们与您取得联系，信息仅用于本次业务沟通</small>
            {message ? <p className={state === "success" ? styles.contactSuccess : styles.contactError} role="status">{message}</p> : null}
          </form>
        </div>
      </section>
    </div> : null}
  </>;
}
