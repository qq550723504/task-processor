"use client";

import { useCallback, useEffect, useState } from "react";

import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  clearSDSLoginState,
  getSDSLoginAuthState,
  getSDSLoginStatus,
  manualSDSLogin,
  triggerSDSLogin,
  type SDSLoginAuthState,
  type SDSLoginStatus,
} from "@/lib/api/sds-login";

function formatDateTime(value?: string) {
  if (!value) {
    return "未知";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function summarizeStatus(status: SDSLoginStatus | null) {
  if (!status) {
    return "正在读取 SDS 登录状态。";
  }
  if (status.login_in_progress) {
    return "后台正在尝试恢复 SDS 登录态。";
  }
  if (status.waiting_for_verify_code) {
    return "当前 SDS 登录停在验证码阶段，需要继续人工处理。";
  }
  if (status.has_access_token) {
    return "当前 SDS 登录态可用，可继续 baseline 校验或生成。";
  }
  if (status.last_error?.trim()) {
    return `当前 SDS 登录态异常：${status.last_error.trim()}`;
  }
  if (!status.has_access_token) {
    return "当前 SDS 登录缺少 access token，需要重新建立登录态。";
  }
  return "当前 SDS 登录状态需要进一步确认。";
}

export function SdsLoginPage() {
  const [status, setStatus] = useState<SDSLoginStatus | null>(null);
  const [authState, setAuthState] = useState<SDSLoginAuthState | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isTriggeringLogin, setIsTriggeringLogin] = useState(false);
  const [isSubmittingManualLogin, setIsSubmittingManualLogin] = useState(false);
  const [isClearing, setIsClearing] = useState(false);
  const [feedback, setFeedback] = useState("");
  const [error, setError] = useState("");
  const [tenantID, setTenantID] = useState("");
  const [identifier, setIdentifier] = useState("");
  const [merchantName, setMerchantName] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError("");
    try {
      const [nextStatus, nextAuthState] = await Promise.all([
        getSDSLoginStatus(),
        getSDSLoginAuthState(),
      ]);
      setStatus(nextStatus);
      setAuthState(nextAuthState);
      setTenantID((current) => current || nextStatus.tenant_id || nextAuthState?.tenant_id || "");
      setIdentifier(
        (current) => current || nextStatus.identifier || nextAuthState?.identifier || "",
      );
      setMerchantName(
        (current) =>
          current || nextStatus.merchant_name || nextAuthState?.merchant_name || "",
      );
      setUsername(
        (current) => current || nextStatus.username || nextAuthState?.username || "",
      );
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : String(nextError));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleTriggerLogin = useCallback(async () => {
    setIsTriggeringLogin(true);
    setFeedback("");
    setError("");
    try {
      await triggerSDSLogin();
      setFeedback("已发起 SDS 自动登录，请稍后刷新状态查看是否恢复。");
      await refresh();
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : String(nextError));
    } finally {
      setIsTriggeringLogin(false);
    }
  }, [refresh]);

  const handleClearState = useCallback(async () => {
    setIsClearing(true);
    setFeedback("");
    setError("");
    try {
      await clearSDSLoginState();
      setFeedback("已清理 SDS 登录状态。可以重新发起登录。");
      await refresh();
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : String(nextError));
    } finally {
      setIsClearing(false);
    }
  }, [refresh]);

  const handleManualLogin = useCallback(async () => {
    setIsSubmittingManualLogin(true);
    setFeedback("");
    setError("");
    try {
      await manualSDSLogin({
        tenantID: tenantID.trim(),
        identifier: identifier.trim(),
        merchantName: merchantName.trim(),
        username: username.trim(),
        password,
      });
      setPassword("");
      setFeedback("已提交 SDS 手动登录，请稍后刷新状态查看是否恢复。");
      await refresh();
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : String(nextError));
    } finally {
      setIsSubmittingManualLogin(false);
    }
  }, [identifier, merchantName, password, refresh, tenantID, username]);

  return (
    <ListingKitPageShell contentClassName="mx-auto w-full max-w-5xl px-4 lg:px-6">
      <div className="space-y-6">
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            SDS Login
          </p>
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-950">
            处理 SDS 登录状态
          </h1>
          <p className="max-w-3xl text-sm leading-7 text-zinc-600">
            当 baseline 校验提示缺少 access token 时，先在这里恢复 SDS 登录态，再回到批次继续生成或校验。
          </p>
        </div>

        <Card className="space-y-4 rounded-3xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-zinc-950">当前状态</h2>
              <p className="text-sm text-zinc-600">
                {isLoading ? "正在同步 SDS 登录状态..." : summarizeStatus(status)}
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button onClick={() => void refresh()} type="button" variant="secondary">
                刷新状态
              </Button>
              <Button
                onClick={() => void handleTriggerLogin()}
                type="button"
                disabled={isTriggeringLogin}
              >
                {isTriggeringLogin ? "登录中..." : "发起自动登录"}
              </Button>
              <Button
                onClick={() => void handleClearState()}
                type="button"
                variant="outline"
                disabled={isClearing}
              >
                {isClearing ? "清理中..." : "清理登录状态"}
              </Button>
            </div>
          </div>

          {feedback ? (
            <div className="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900">
              {feedback}
            </div>
          ) : null}
          {error ? (
            <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-900">
              {error}
            </div>
          ) : null}

          <div className="grid gap-3 md:grid-cols-2">
            <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-4 text-sm text-zinc-700">
              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">Access token</span>
                  <span>{status?.has_access_token ? "可用" : "缺失"}</span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">登录进行中</span>
                  <span>{status?.login_in_progress ? "是" : "否"}</span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">验证码等待</span>
                  <span>{status?.waiting_for_verify_code ? "是" : "否"}</span>
                </div>
              </div>
            </div>
            <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-4 text-sm text-zinc-700">
              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">租户</span>
                  <span>{status?.tenant_id || "未配置"}</span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">Identifier</span>
                  <span>{status?.identifier || "未配置"}</span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">商家名</span>
                  <span>{status?.merchant_name || "未知"}</span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">签发时间</span>
                  <span>{formatDateTime(status?.issued_at)}</span>
                </div>
              </div>
            </div>
          </div>

          <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-4">
            <p className="text-sm font-medium text-zinc-900">最近持有的认证信息</p>
            <div className="mt-3 grid gap-2 text-sm text-zinc-700 md:grid-cols-2">
              <div className="flex items-center justify-between gap-3">
                <span className="text-zinc-500">Access token</span>
                <span>{authState?.access_token ? "已缓存" : "无"}</span>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-zinc-500">Cookie 数量</span>
                <span>{authState?.cookies?.length ?? 0}</span>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-zinc-500">来源</span>
                <span>{authState?.source || "未知"}</span>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-zinc-500">当前 URL</span>
                <span className="truncate text-right">{authState?.current_url || "未知"}</span>
              </div>
            </div>
          </div>

          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="space-y-1">
              <p className="text-sm font-medium text-zinc-900">手动登录 SDS</p>
              <p className="text-sm text-zinc-600">
                当自动登录因为本地未配置账号而不可用时，可以在这里直接提交一组全局 SDS 登录凭证。
              </p>
            </div>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <label className="space-y-2">
                <Label htmlFor="sds-login-tenant-id">租户</Label>
                <Input
                  id="sds-login-tenant-id"
                  onChange={(event) => setTenantID(event.target.value)}
                  value={tenantID}
                />
              </label>
              <label className="space-y-2">
                <Label htmlFor="sds-login-identifier">Identifier</Label>
                <Input
                  id="sds-login-identifier"
                  onChange={(event) => setIdentifier(event.target.value)}
                  value={identifier}
                />
              </label>
              <label className="space-y-2">
                <Label htmlFor="sds-login-merchant-name">商家名</Label>
                <Input
                  id="sds-login-merchant-name"
                  onChange={(event) => setMerchantName(event.target.value)}
                  value={merchantName}
                />
              </label>
              <label className="space-y-2">
                <Label htmlFor="sds-login-username">用户名</Label>
                <Input
                  id="sds-login-username"
                  onChange={(event) => setUsername(event.target.value)}
                  value={username}
                />
              </label>
              <label className="space-y-2 md:col-span-2">
                <Label htmlFor="sds-login-password">密码</Label>
                <Input
                  id="sds-login-password"
                  onChange={(event) => setPassword(event.target.value)}
                  type="password"
                  value={password}
                />
              </label>
            </div>
            <div className="mt-4 flex flex-wrap gap-2">
              <Button
                disabled={isSubmittingManualLogin}
                onClick={() => void handleManualLogin()}
                type="button"
              >
                {isSubmittingManualLogin ? "提交中..." : "提交手动登录"}
              </Button>
            </div>
          </div>
        </Card>
      </div>
    </ListingKitPageShell>
  );
}
