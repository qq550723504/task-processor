"use client";

import { FormEvent, useState } from "react";

import styles from "./marketing-homepage.module.css";

const PLATFORM_OPTIONS = ["SHEIN", "TEMU", "Amazon", "TikTok Shop"];

export function DemoRequestForm() {
  const [platforms, setPlatforms] = useState<string[]>([]);
  const [state, setState] = useState<"idle" | "submitting" | "success" | "error">("idle");
  const [message, setMessage] = useState("");

  function togglePlatform(platform: string) {
    setPlatforms((current) => current.includes(platform)
      ? current.filter((item) => item !== platform)
      : [...current, platform]);
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (platforms.length === 0) {
      setState("error");
      setMessage("请至少选择一个目标渠道。");
      return;
    }

    const form = event.currentTarget;
    const data = new FormData(form);
    setState("submitting");
    setMessage("");

    try {
      const response = await fetch("/api/demo-requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: data.get("name"),
          company: data.get("company"),
          contact: data.get("contact"),
          message: data.get("message"),
          platforms,
        }),
      });
      if (!response.ok) throw new Error("Demo request failed");
      form.reset();
      setPlatforms([]);
      setState("success");
      setMessage("预约已提交，我们会尽快与你联系。");
    } catch {
      setState("error");
      setMessage("提交未成功，请稍后重试。");
    }
  }

  return (
    <form className={styles.demoForm} onSubmit={submit}>
      <div className={styles.demoFormGrid}>
        <label>姓名<input autoComplete="name" name="name" placeholder="怎么称呼你" required /></label>
        <label>公司或店铺<input autoComplete="organization" name="company" placeholder="公司 / 店铺名称" required /></label>
        <label className={styles.demoFormWide}>联系方式<input autoComplete="email" name="contact" placeholder="邮箱、微信或手机号码" required /></label>
      </div>
      <fieldset><legend>关注的目标渠道 <em>*</em></legend><div className={styles.platformChoices}>{PLATFORM_OPTIONS.map((platform) => <label key={platform}><input checked={platforms.includes(platform)} onChange={() => togglePlatform(platform)} type="checkbox" /><span>{platform}</span></label>)}</div></fieldset>
      <label className={styles.messageField}>想解决的问题 <textarea maxLength={1000} name="message" placeholder="例如：希望将现有商品资料同步到多个渠道" rows={3} /></label>
      <div className={styles.formFooter}><p>提交即表示你同意我们就本次演示需求联系你。</p><button disabled={state === "submitting"} type="submit">{state === "submitting" ? "提交中…" : "提交预约"}</button></div>
      {message ? <p className={`${styles.formMessage} ${state === "success" ? styles.formSuccess : styles.formError}`} role="status">{message}</p> : null}
    </form>
  );
}
