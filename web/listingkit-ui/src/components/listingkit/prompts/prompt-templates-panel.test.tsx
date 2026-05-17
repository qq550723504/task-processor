import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PromptTemplatesPanel } from "@/components/listingkit/prompts/prompt-templates-panel";

const upsertMock = vi.fn();
const statusMock = vi.fn();
const promptQueryState = {
  isError: false,
};
const promptCatalogState = {
  isError: false,
};
const upsertState = {
  error: null as Error | null,
};

vi.mock("@/lib/query/use-prompt-management", () => ({
  usePromptTemplateCatalog: () => ({
    data: {
      items: [
        {
          key: "shein.content_optimizer.optimize_title_description_system",
          label: "Optimize Title Description System",
          description: "SHEIN 提示词模板",
          group: "marketplace",
          group_label: "平台运营",
          category: "shein",
          category_label: "SHEIN",
          supported_scopes: [{ id: "tenant", label: "租户" }],
          variables: [
            {
              key: "title",
              label: "标题",
              description: "商品或 listing 的主标题文本。",
            },
          ],
          has_default_content: true,
          supports_tenant_override: true,
        },
        {
          key: "productimage.studio_generation.pod_design",
          label: "POD Design",
          description: "图片提示词模板",
          group: "image",
          group_label: "图片生成",
          category: "productimage",
          category_label: "商品图片",
          supported_scopes: [{ id: "tenant", label: "租户" }],
          variables: [{ key: "ThemePrompt", label: "Theme Prompt" }],
          has_default_content: true,
          supports_tenant_override: true,
        },
        {
          key: "temu.content_rewriter.system",
          label: "Content Rewriter System",
          description: "Temu 提示词模板",
          group: "marketplace",
          group_label: "平台运营",
          category: "temu",
          category_label: "Temu",
          supported_scopes: [{ id: "tenant", label: "租户" }],
          variables: [],
          has_default_content: false,
          supports_tenant_override: true,
        },
      ],
    },
    isLoading: false,
    isError: promptCatalogState.isError,
  }),
  usePromptTemplates: () => ({
    data: {
      items: [
        {
          key: "shein.content_optimizer.optimize_title_description_system",
          content: "System prompt",
          version: "v1",
          enabled: true,
        },
        {
          key: "productimage.studio_generation.pod_design",
          content: "Disabled override",
          version: "v2",
          enabled: false,
        },
      ],
    },
    isLoading: false,
    isError: promptQueryState.isError,
  }),
  useUpsertPromptTemplate: () => ({
    mutate: upsertMock,
    isPending: false,
    error: upsertState.error,
  }),
  useSetPromptTemplateStatus: () => ({
    mutate: statusMock,
    isPending: false,
    error: null,
  }),
}));

describe("PromptTemplatesPanel", () => {
  beforeEach(() => {
    promptQueryState.isError = false;
    promptCatalogState.isError = false;
    upsertState.error = null;
    upsertMock.mockClear();
    statusMock.mockClear();
  });

  it("loads a prompt into the editor and saves changes", () => {
    render(<PromptTemplatesPanel />);

    expect(screen.getAllByText("平台运营").length).toBeGreaterThan(0);
    expect(screen.getAllByText("SHEIN").length).toBeGreaterThan(0);
    expect(screen.getByText("选择一个模板开始编辑")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", {
        name: /shein.content_optimizer.optimize_title_description_system/,
      }),
    );
    expect(screen.getByText("商品或 listing 的主标题文本。")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Prompt 内容"), {
      target: { value: "Updated prompt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存提示词" }));

    expect(upsertMock).toHaveBeenCalledWith(
      expect.objectContaining({
        key: "shein.content_optimizer.optimize_title_description_system",
        content: "Updated prompt",
        version: "v1",
        enabled: true,
      }),
    );
  });

  it("renders prompt failures as alerts", () => {
    promptQueryState.isError = true;
    promptCatalogState.isError = true;
    upsertState.error = new Error("save failed");

    render(<PromptTemplatesPanel />);

    expect(screen.getAllByRole("status")).toHaveLength(2);
  });

  it("filters templates by group", () => {
    render(<PromptTemplatesPanel />);

    fireEvent.change(screen.getByLabelText("用途分组"), {
      target: { value: "image" },
    });

    expect(screen.getByRole("button", { name: /POD Design/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: /Optimize Title Description System/i,
      }),
    ).not.toBeInTheDocument();
  });

  it("filters templates by disabled overrides", () => {
    render(<PromptTemplatesPanel />);

    fireEvent.change(screen.getByLabelText("覆盖状态"), {
      target: { value: "overridden_disabled" },
    });

    expect(screen.getByRole("button", { name: /POD Design/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: /Optimize Title Description System/i,
      }),
    ).not.toBeInTheDocument();
  });

  it("filters templates by missing default content", () => {
    render(<PromptTemplatesPanel />);

    fireEvent.change(screen.getByLabelText("默认模板"), {
      target: { value: "missing" },
    });

    expect(screen.getByRole("button", { name: /Content Rewriter System/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: /Optimize Title Description System/i,
      }),
    ).not.toBeInTheDocument();
  });

  it("filters templates without variables", () => {
    render(<PromptTemplatesPanel />);

    fireEvent.change(screen.getByLabelText("模板变量"), {
      target: { value: "without_variables" },
    });

    expect(screen.getByRole("button", { name: /Content Rewriter System/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: /POD Design/i,
      }),
    ).not.toBeInTheDocument();
  });

  it("assigns stable form ids and names to prompt catalog filters", () => {
    render(<PromptTemplatesPanel />);

    expect(screen.getByLabelText("搜索提示词模板")).toHaveAttribute("id", "prompt-template-search");
    expect(screen.getByLabelText("搜索提示词模板")).toHaveAttribute("name", "prompt-template-search");
    expect(screen.getByLabelText("用途分组")).toHaveAttribute("id", "prompt-template-group-filter");
    expect(screen.getByLabelText("用途分组")).toHaveAttribute("name", "prompt-template-group-filter");
    expect(screen.getByLabelText("覆盖状态")).toHaveAttribute("id", "prompt-template-coverage-filter");
    expect(screen.getByLabelText("覆盖状态")).toHaveAttribute("name", "prompt-template-coverage-filter");
    expect(screen.getByLabelText("默认模板")).toHaveAttribute(
      "id",
      "prompt-template-default-content-filter",
    );
    expect(screen.getByLabelText("默认模板")).toHaveAttribute(
      "name",
      "prompt-template-default-content-filter",
    );
    expect(screen.getByLabelText("模板变量")).toHaveAttribute("id", "prompt-template-variable-filter");
    expect(screen.getByLabelText("模板变量")).toHaveAttribute(
      "name",
      "prompt-template-variable-filter",
    );
  });
});
