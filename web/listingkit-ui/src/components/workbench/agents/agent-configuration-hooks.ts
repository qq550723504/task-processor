"use client";
import { useEffect, useState } from "react";
import { z } from "zod";
import { configurationRequest, ConfigurationError, type ConfigurationScope } from "@/lib/api/agent-configuration";

export function configurationError(error: unknown) {
  const code = error instanceof ConfigurationError ? error.code : "DEPENDENCY_UNAVAILABLE";
  return ({
    FORBIDDEN: "当前身份没有操作权限。",
    IDENTITY_CONTEXT_CHANGED: "登录身份已变化，请重新确认。",
    ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化，请重新确认。",
    REVISION_MISMATCH: "配置已被更新，请刷新当前状态后再次确认。",
    TEMPLATE_IS_DEFAULT: "此模板仍是企业默认，请先清除或替换默认模板。",
    TEMPLATE_ARCHIVED: "模板已归档，无法再用于新执行。",
    IDEMPOTENCY_CONFLICT: "本次请求编号已用于其他内容，请刷新后重新确认。",
    CONFIGURATION_CHANGED: "配置准入已变化，请重新确认；不会自动发起新执行。",
    AGENT_NOT_ENABLED: "请先由管理员在当前企业启用智能体。",
    OUTCOME_UNKNOWN: "操作结果尚未确认，请核实原操作以读取原回执。",
    INVALID_REQUEST: "请检查名称、平台、参数和版本。",
    NOT_FOUND: "该配置在当前企业不可用。",
  } as Record<string, string>)[code] ?? "当前能力或依赖不可用，请稍后重试。";
}

export function useConfigurationRead<T>(scope: ConfigurationScope, path: string, schema: z.ZodType<T>, nonce = 0, enabled = true) {
  const { userId, organizationId } = scope;
  const [result, setResult] = useState<{ path: string; data?: T; error?: unknown }>({ path: "" });
  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    configurationRequest({ userId, organizationId }, path, schema, { signal: controller.signal })
      .then(data => { if (!controller.signal.aborted) setResult({ path: `${path}:${nonce}`, data }); })
      .catch(error => { if (!controller.signal.aborted) setResult({ path: `${path}:${nonce}`, error }); });
    return () => controller.abort();
  }, [userId, organizationId, path, schema, nonce, enabled]);
  return result.path === `${path}:${nonce}` && enabled ? result : { path };
}
