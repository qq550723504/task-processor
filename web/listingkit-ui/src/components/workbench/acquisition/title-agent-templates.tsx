"use client";
import { useEffect, useRef, useState } from "react";
import {
  configurationRequest,
  type ConfigurationScope,
} from "@/lib/api/agent-configuration";
import {
  catalogEntrySchema,
  templateSchema,
  templatesPageSchema,
  configVersion,
  type AgentTemplate,
} from "@/lib/contracts/agent-configuration";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function TitleAgentTemplates({
  scope,
  onChoose,
  onReady,
}: {
  scope: ConfigurationScope;
  onChoose: (template: AgentTemplate | null) => void;
  onReady: (ready: boolean) => void;
}) {
  const { userId, organizationId } = scope;
  const [items, setItems] = useState<AgentTemplate[]>([]),
    [selected, setSelected] = useState(""),
    [version, setVersion] = useState(""),
    [next, setNext] = useState(""),
    [busy, setBusy] = useState(true),
    [failure, setFailure] = useState("");
  const current = useRef<AbortController | null>(null),
    generation = useRef(0),
    permitted = useRef(false),
    callbacks = useRef({ onChoose, onReady });
  useEffect(() => {
    callbacks.current = { onChoose, onReady };
  }, [onChoose, onReady]);
  useEffect(() => {
    const controller = new AbortController();
    current.current = controller;
    const token = ++generation.current;
    callbacks.current.onReady(false);
    void (async () => {
      try {
        const entry = await configurationRequest(
          { userId, organizationId },
          "product.title.agent",
          catalogEntrySchema,
          { signal: controller.signal },
        );
        if (controller.signal.aborted || generation.current !== token) return;
        if (!entry.canUse) {
          setFailure(
            "当前企业智能体未启用，或当前身份、文本能力不满足执行条件。",
          );
          return;
        }
        permitted.current = true;
        const page = await configurationRequest(
          { userId, organizationId },
          "product.title.agent/templates?lifecycle=ACTIVE&pageSize=100",
          templatesPageSchema,
          { signal: controller.signal },
        );
        if (controller.signal.aborted || generation.current !== token) return;
        setItems(page.items);
        setNext(page.nextCursor);
        if (entry.agent.defaultTemplate) {
          const ref = entry.agent.defaultTemplate;
          setSelected(ref.templateId);
          setVersion(ref.revision);
          try {
            const template = await configurationRequest(
              { userId, organizationId },
              `product.title.agent/templates/${ref.templateId}/revisions/${ref.revision}`,
              templateSchema,
              { signal: controller.signal },
            );
            if (controller.signal.aborted || generation.current !== token)
              return;
            if (template.lifecycle !== "ACTIVE") throw Error("archived");
            setItems((old) =>
              old.some((t) => t.templateId === template.templateId)
                ? old
                : [template, ...old],
            );
            callbacks.current.onChoose(template);
            callbacks.current.onReady(true);
          } catch {
            if (!controller.signal.aborted) {
              setSelected("unresolved");
              setFailure(
                "默认精确版本当前不可用。请选择其他模板，或明确选择不使用模板。",
              );
            }
          }
        } else {
          callbacks.current.onChoose(null);
          callbacks.current.onReady(true);
        }
      } catch {
        if (!controller.signal.aborted) {
          setFailure(
            "无法读取当前企业配置，请从我的智能体重新确认启用状态与权限。",
          );
        }
      } finally {
        if (!controller.signal.aborted) setBusy(false);
      }
    })();
    return () => {
      controller.abort();
    };
  }, [userId, organizationId]);
  async function choose(id: string, v?: string) {
    if (!permitted.current) return;
    current.current?.abort();
    const token = ++generation.current;
    setSelected(id);
    setFailure("");
    callbacks.current.onReady(false);
    if (!id) {
      setVersion("");
      callbacks.current.onChoose(null);
      callbacks.current.onReady(true);
      setBusy(false);
      return;
    }
    const revision = v ?? items.find((t) => t.templateId === id)?.version ?? "";
    setVersion(revision);
    if (!configVersion.safeParse(revision).success) {
      setFailure("请输入完整的正整数版本。");
      return;
    }
    const controller = new AbortController();
    current.current = controller;
    setBusy(true);
    try {
      const template = await configurationRequest(
        { userId, organizationId },
        `product.title.agent/templates/${id}/revisions/${revision}`,
        templateSchema,
        { signal: controller.signal },
      );
      if (controller.signal.aborted || generation.current !== token) return;
      if (template.lifecycle !== "ACTIVE") throw Error("archived");
      callbacks.current.onChoose(template);
      callbacks.current.onReady(true);
    } catch {
      if (!controller.signal.aborted)
        setFailure("该精确模板版本当前不可用，请重新选择。");
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  async function more() {
    if (!next || busy) return;
    const controller = new AbortController();
    current.current = controller;
    setBusy(true);
    try {
      const page = await configurationRequest(
        { userId, organizationId },
        `product.title.agent/templates?lifecycle=ACTIVE&pageSize=100&cursor=${next}`,
        templatesPageSchema,
        { signal: controller.signal },
      );
      if (!controller.signal.aborted) {
        setItems((old) => [
          ...old,
          ...page.items.filter(
            (t) => !old.some((o) => o.templateId === t.templateId),
          ),
        ]);
        setNext(page.nextCursor);
      }
    } catch {
      if (!controller.signal.aborted) setFailure("后续模板读取失败，请重试。");
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <section className="space-y-2">
      <label htmlFor="title-template">标题模板</label>
      <select
        id="title-template"
        className="block w-full rounded-lg border p-2"
        disabled={busy}
        value={selected}
        onChange={(e) => void choose(e.target.value)}
      >
        <option value="">不使用模板</option>
        {selected === "unresolved" && (
          <option value="unresolved" disabled>
            默认版本不可用，请重新选择
          </option>
        )}
        {items.map((t) => (
          <option key={t.templateId} value={t.templateId}>
            {t.name} · 当前 v{t.version}
          </option>
        ))}
      </select>
      {selected && selected !== "unresolved" && (
        <label>
          采用精确版本
          <Input
            inputMode="numeric"
            value={version}
            onChange={(e) => {
              setVersion(e.target.value);
              callbacks.current.onReady(false);
            }}
            onBlur={() => void choose(selected, version)}
          />
        </label>
      )}
      {next && (
        <Button variant="outline" disabled={busy} onClick={() => void more()}>
          读取更多模板
        </Button>
      )}
      {busy && <p role="status">正在读取企业模板…</p>}
      {failure && <p role="alert">{failure}</p>}
      <p className="text-sm">
        模板仅预填平台与知识选择；发送前仍需明确确认，模板新版本不会自动替换本次选择。
      </p>
    </section>
  );
}
