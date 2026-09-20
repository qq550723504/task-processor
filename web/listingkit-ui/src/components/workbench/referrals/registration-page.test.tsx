import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RegistrationPage } from "./registration-page";

afterEach(() => {
  cleanup();
  window.history.replaceState(null, "", "/");
  vi.unstubAllGlobals();
});

describe("RegistrationPage", () => {
  it("collects only the public registration contract and never asks for auth secrets", () => {
    render(<RegistrationPage code="CODE1234" />);
    expect(screen.getByRole("textbox", { name: "邀请码" })).toHaveValue("CODE1234");
    expect(screen.getByRole("textbox", { name: "邮箱" })).toBeRequired();
    expect(screen.getByRole("textbox", { name: "名字" })).toBeRequired();
    expect(screen.getByRole("textbox", { name: "姓氏" })).toBeRequired();
    expect(screen.queryByLabelText(/密码|验证码|OTP/i)).not.toBeInTheDocument();
  });

  it("continues the admitted intent through the same explicit submit action", async () => {
    const admission = { intentID: "intent-1", resumeSecret: "a".repeat(64), createExpiresAt: "2026-09-13T10:15:00Z", completionExpiresAt: "2026-09-14T10:00:00Z" };
    const fetch = vi.fn()
      .mockResolvedValueOnce(Response.json(admission))
      .mockResolvedValueOnce(Response.json({ status: "created" }));
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    render(<RegistrationPage code="CODE1234" />);
    await user.type(screen.getByRole("textbox", { name: "邮箱" }), "new@example.test");
    await user.type(screen.getByRole("textbox", { name: "名字" }), "新");
    await user.type(screen.getByRole("textbox", { name: "姓氏" }), "用户");
    await user.click(screen.getByRole("button", { name: "开始注册" }));
    expect(await screen.findByText("请查看官方验证邮件")).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(fetch.mock.calls[0][0]).toBe("/api/referral-registration");
    expect(fetch.mock.calls[1][0]).toBe("/api/referral-registration/resume");
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ intentID: "intent-1", resumeSecret: "a".repeat(64) });
  });

  it("reuses one high-entropy key after a lost response and puts recovery only in the URL fragment", async () => {
    const admission = { intentID: "intent-1", resumeSecret: "a".repeat(64), createExpiresAt: "2026-09-13T10:15:00Z", completionExpiresAt: "2026-09-14T10:00:00Z" };
    const fetch = vi.fn()
      .mockRejectedValueOnce(new TypeError("response lost"))
      .mockResolvedValueOnce(Response.json(admission))
      .mockResolvedValueOnce(Response.json({ status: "created" }));
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    render(<RegistrationPage code="CODE1234" />);
    await user.type(screen.getByRole("textbox", { name: "邮箱" }), "new@example.test");
    await user.type(screen.getByRole("textbox", { name: "名字" }), "新");
    await user.type(screen.getByRole("textbox", { name: "姓氏" }), "用户");
    await user.click(screen.getByRole("button", { name: "开始注册" }));
    expect(await screen.findByText("暂时无法确认注册结果")).toBeVisible();
    expect(screen.getByRole("textbox", { name: "邮箱" })).toHaveAttribute("readonly");
    await user.click(screen.getByRole("button", { name: "重试原请求" }));
    expect(await screen.findByText("请查看官方验证邮件")).toBeVisible();
    const keys = fetch.mock.calls.map(([, init]) => init.headers["Idempotency-Key"]);
    expect(keys[0]).toMatch(/^[A-Za-z0-9_-]{43,128}$/);
    expect(keys[1]).toBe(keys[0]);
    expect(fetch.mock.calls[1][1].body).toBe(fetch.mock.calls[0][1].body);
    expect(window.location.hash).toContain("intentID=intent-1");
    expect(window.location.hash).toContain(`resumeSecret=${"a".repeat(64)}`);
    expect(screen.getByRole("link", { name: "已完成验证，继续登录" })).toHaveAttribute("href", "/login?returnTo=%2Fworkbench%2Faccount%2Freferrals%2Fcomplete");
    expect(document.body.textContent).not.toContain("a".repeat(64));
  });

  it("distinguishes an expired operation from an unknown outcome", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "referral_expired" }, { status: 409 })));
    const user = userEvent.setup();
    render(<RegistrationPage code="CODE1234" />);
    await user.type(screen.getByRole("textbox", { name: "邮箱" }), "new@example.test");
    await user.type(screen.getByRole("textbox", { name: "名字" }), "新");
    await user.type(screen.getByRole("textbox", { name: "姓氏" }), "用户");
    await user.click(screen.getByRole("button", { name: "开始注册" }));
    expect(await screen.findByText("注册确认期限已结束")).toBeVisible();
    expect(screen.queryByText("暂时无法确认注册结果")).not.toBeInTheDocument();
  });

  it("resumes only from an opaque fragment through an explicit action", async () => {
    window.history.replaceState(null, "", `/#intentID=intent-1&resumeSecret=${"c".repeat(64)}`);
    const fetch = vi.fn().mockResolvedValue(Response.json({ status: "created" }));
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    render(<RegistrationPage code="CODE1234" />);
    expect(fetch).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "恢复原注册" }));
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(fetch.mock.calls[0][0]).toBe("/api/referral-registration/resume");
    expect(screen.getByText("请查看官方验证邮件")).toBeVisible();
  });

  it.each([["referral_expired", "注册确认期限已结束"], ["referral_conflict", "注册请求存在冲突"]])("stops fragment recovery for %s", async (code, message) => {
    window.history.replaceState(null, "", `/#intentID=intent-1&resumeSecret=${"c".repeat(64)}`);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code }, { status: 409 })));
    const user = userEvent.setup();
    render(<RegistrationPage code="CODE1234" />);
    await user.click(screen.getByRole("button", { name: "恢复原注册" }));
    expect(await screen.findByText(message)).toBeVisible();
    expect(screen.queryByRole("button", { name: "恢复原注册" })).not.toBeInTheDocument();
  });

  it("stops automatic resume on a conflict without relabeling it unknown", async () => {
    const admission = { intentID: "intent-1", resumeSecret: "a".repeat(64), createExpiresAt: "2026-09-13T10:15:00Z", completionExpiresAt: "2026-09-14T10:00:00Z" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(Response.json(admission)).mockResolvedValueOnce(Response.json({ code: "referral_conflict" }, { status: 409 })));
    const user = userEvent.setup();
    render(<RegistrationPage code="CODE1234" />);
    await user.type(screen.getByRole("textbox", { name: "邮箱" }), "new@example.test");
    await user.type(screen.getByRole("textbox", { name: "名字" }), "新");
    await user.type(screen.getByRole("textbox", { name: "姓氏" }), "用户");
    await user.click(screen.getByRole("button", { name: "开始注册" }));
    expect(await screen.findByText("注册请求存在冲突")).toBeVisible();
    expect(screen.queryByRole("button", { name: "恢复原注册" })).not.toBeInTheDocument();
  });

  it("keeps an unknown resume on the original intent and never restarts registration", async () => {
    const admission = { intentID: "intent-1", resumeSecret: "d".repeat(64), createExpiresAt: "2026-09-13T10:15:00Z", completionExpiresAt: "2026-09-14T10:00:00Z" };
    const fetch = vi.fn()
      .mockResolvedValueOnce(Response.json(admission))
      .mockResolvedValueOnce(Response.json({ code: "referral_outcome_unknown" }, { status: 503 }))
      .mockResolvedValueOnce(Response.json({ status: "created" }));
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    render(<RegistrationPage code="CODE1234" />);
    await user.type(screen.getByRole("textbox", { name: "邮箱" }), "new@example.test");
    await user.type(screen.getByRole("textbox", { name: "名字" }), "新");
    await user.type(screen.getByRole("textbox", { name: "姓氏" }), "用户");
    await user.click(screen.getByRole("button", { name: "开始注册" }));
    expect(await screen.findByText("暂时无法确认注册结果")).toBeVisible();
    expect(screen.queryByText("请查看官方验证邮件")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /开始注册|重试原请求/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "恢复原注册" }));
    expect(await screen.findByText("请查看官方验证邮件")).toBeVisible();
    expect(fetch.mock.calls.map(([url]) => url)).toEqual([
      "/api/referral-registration",
      "/api/referral-registration/resume",
      "/api/referral-registration/resume",
    ]);
    expect(fetch.mock.calls[2][1].body).toBe(fetch.mock.calls[1][1].body);
  });

  it("stops before submission when the public code is invalid", () => {
    render(<RegistrationPage code="" />);
    expect(screen.getByText("邀请链接无效或已缺少邀请码")).toBeVisible();
    expect(screen.queryByRole("button", { name: "开始注册" })).not.toBeInTheDocument();
  });
});
