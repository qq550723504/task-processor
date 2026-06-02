import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { GenerationTopicPolicyAdminPage } from "@/components/listingkit/admin/generation-topic-policy-admin-page";
import * as generationTopicPoliciesApi from "@/lib/api/admin-generation-topic-policies";
import * as generationTopicOverridesApi from "@/lib/api/admin-generation-topic-overrides";

describe("GenerationTopicPolicyAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit generation topic policies", async () => {
    vi.spyOn(
      generationTopicPoliciesApi,
      "getListingGenerationTopicPolicies",
    ).mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          platform: "shein",
          topicKey: "children",
          remark: "manual",
          status: 1,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(
      generationTopicOverridesApi,
      "getListingGenerationTopicCatalog",
    ).mockResolvedValue({
      items: [
        {
          key: "children",
          priority: 10,
          promptDirectives: [
            "Do not mention children, babies, or age-specific users.",
          ],
          lexiconByLanguage: { en: ["child", "children"], zh: ["儿童"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: [
              "Do not mention children, babies, or age-specific users.",
            ],
            lexiconByLanguage: { en: ["child", "children"], zh: ["儿童"] },
          },
        },
      ],
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <GenerationTopicPolicyAdminPage />
      </QueryClientProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "生成禁用主题" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("manual")).toBeInTheDocument();
    });
    expect(await screen.findByText("Topic Catalog")).toBeInTheDocument();
    expect(screen.getAllByText("child, children")).toHaveLength(2);
    expect(
      generationTopicPoliciesApi.getListingGenerationTopicPolicies,
    ).toHaveBeenCalled();
    expect(generationTopicOverridesApi.getListingGenerationTopicCatalog).toHaveBeenCalled();
    expect(screen.getByText("manual")).toBeInTheDocument();
  });

  it("shows one selected topic editor at a time and lets users switch topics", async () => {
    vi.spyOn(
      generationTopicPoliciesApi,
      "getListingGenerationTopicPolicies",
    ).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(
      generationTopicOverridesApi,
      "getListingGenerationTopicCatalog",
    ).mockResolvedValue({
      items: [
        {
          key: "children",
          priority: 10,
          promptDirectives: ["Do not mention children."],
          lexiconByLanguage: { en: ["child"], zh: ["儿童"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: ["Do not mention children."],
            lexiconByLanguage: { en: ["child"], zh: ["儿童"] },
          },
        },
        {
          key: "knives",
          priority: 30,
          promptDirectives: ["Do not mention knives."],
          lexiconByLanguage: { en: ["knife"], zh: ["刀具"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: ["Do not mention knives."],
            lexiconByLanguage: { en: ["knife"], zh: ["刀具"] },
          },
        },
      ],
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <GenerationTopicPolicyAdminPage />
      </QueryClientProvider>,
    );

    expect(
      await screen.findByRole("button", { name: /children/i }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Do not mention children.")).toHaveLength(2);
    expect(screen.queryByText("Do not mention knives.")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /knives/i }));

    await waitFor(() => {
      expect(screen.getAllByText("Do not mention knives.")).toHaveLength(2);
    });
    expect(screen.queryByText("Do not mention children.")).not.toBeInTheDocument();
  });

  it("opens the create policy dialog on demand instead of keeping the form always visible", async () => {
    vi.spyOn(
      generationTopicPoliciesApi,
      "getListingGenerationTopicPolicies",
    ).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(
      generationTopicOverridesApi,
      "getListingGenerationTopicCatalog",
    ).mockResolvedValue({
      items: [
        {
          key: "children",
          priority: 10,
          promptDirectives: ["Do not mention children."],
          lexiconByLanguage: { en: ["child"], zh: ["儿童"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: ["Do not mention children."],
            lexiconByLanguage: { en: ["child"], zh: ["儿童"] },
          },
        },
      ],
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <GenerationTopicPolicyAdminPage />
      </QueryClientProvider>,
    );

    expect(
      screen.queryByRole("heading", { name: "新增生成主题策略" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /新增策略/i }));

    expect(
      await screen.findByRole("heading", { name: "新增生成主题策略" }),
    ).toBeInTheDocument();
  });

  it("orders enabled overrides before disabled and unconfigured topics", async () => {
    vi.spyOn(
      generationTopicPoliciesApi,
      "getListingGenerationTopicPolicies",
    ).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(
      generationTopicOverridesApi,
      "getListingGenerationTopicCatalog",
    ).mockResolvedValue({
      items: [
        {
          key: "meals",
          priority: 20,
          promptDirectives: ["Do not mention meals."],
          lexiconByLanguage: { en: ["meal"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: ["Do not mention meals."],
            lexiconByLanguage: { en: ["meal"] },
          },
        },
        {
          key: "children",
          priority: 10,
          promptDirectives: ["Do not mention children."],
          lexiconByLanguage: { en: ["child"] },
          tenantOverride: {
            id: 1,
            status: 1,
            remark: "",
            additionalPromptDirectives: [],
            additionalLexiconByLanguage: {},
          },
          effectiveDefinition: {
            promptDirectives: ["Do not mention children."],
            lexiconByLanguage: { en: ["child"] },
          },
        },
        {
          key: "knives",
          priority: 30,
          promptDirectives: ["Do not mention knives."],
          lexiconByLanguage: { en: ["knife"] },
          tenantOverride: {
            id: 2,
            status: 0,
            remark: "",
            additionalPromptDirectives: [],
            additionalLexiconByLanguage: {},
          },
          effectiveDefinition: {
            promptDirectives: ["Do not mention knives."],
            lexiconByLanguage: { en: ["knife"] },
          },
        },
      ],
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const { container } = render(
      <QueryClientProvider client={queryClient}>
        <GenerationTopicPolicyAdminPage />
      </QueryClientProvider>,
    );

    await screen.findByRole("button", { name: /children/i });

    const topicButtons = Array.from(
      container.querySelectorAll("button[data-topic-key]"),
    ).map((button) => button.getAttribute("data-topic-key"));

    expect(topicButtons).toEqual(["children", "knives", "meals"]);
  });

  it("shows quick stats for enabled, disabled, and unconfigured topics", async () => {
    vi.spyOn(
      generationTopicPoliciesApi,
      "getListingGenerationTopicPolicies",
    ).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(
      generationTopicOverridesApi,
      "getListingGenerationTopicCatalog",
    ).mockResolvedValue({
      items: [
        {
          key: "children",
          priority: 10,
          promptDirectives: ["Do not mention children."],
          lexiconByLanguage: { en: ["child"] },
          tenantOverride: {
            id: 1,
            status: 1,
            remark: "",
            additionalPromptDirectives: [],
            additionalLexiconByLanguage: {},
          },
          effectiveDefinition: {
            promptDirectives: ["Do not mention children."],
            lexiconByLanguage: { en: ["child"] },
          },
        },
        {
          key: "knives",
          priority: 30,
          promptDirectives: ["Do not mention knives."],
          lexiconByLanguage: { en: ["knife"] },
          tenantOverride: {
            id: 2,
            status: 0,
            remark: "",
            additionalPromptDirectives: [],
            additionalLexiconByLanguage: {},
          },
          effectiveDefinition: {
            promptDirectives: ["Do not mention knives."],
            lexiconByLanguage: { en: ["knife"] },
          },
        },
        {
          key: "meals",
          priority: 20,
          promptDirectives: ["Do not mention meals."],
          lexiconByLanguage: { en: ["meal"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: ["Do not mention meals."],
            lexiconByLanguage: { en: ["meal"] },
          },
        },
      ],
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <GenerationTopicPolicyAdminPage />
      </QueryClientProvider>,
    );

    expect(await screen.findByText("启用中 1")).toBeInTheDocument();
    expect(screen.getByText("已禁用 1")).toBeInTheDocument();
    expect(screen.getByText("未配置 1")).toBeInTheDocument();
  });

  it("shows topic usage guidance for the selected topic", async () => {
    vi.spyOn(
      generationTopicPoliciesApi,
      "getListingGenerationTopicPolicies",
    ).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(
      generationTopicOverridesApi,
      "getListingGenerationTopicCatalog",
    ).mockResolvedValue({
      items: [
        {
          key: "children",
          priority: 10,
          promptDirectives: ["Do not mention children."],
          lexiconByLanguage: { en: ["child"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: ["Do not mention children."],
            lexiconByLanguage: { en: ["child"] },
          },
        },
        {
          key: "knives",
          priority: 30,
          promptDirectives: ["Do not mention knives."],
          lexiconByLanguage: { en: ["knife"] },
          tenantOverride: null,
          effectiveDefinition: {
            promptDirectives: ["Do not mention knives."],
            lexiconByLanguage: { en: ["knife"] },
          },
        },
      ],
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <GenerationTopicPolicyAdminPage />
      </QueryClientProvider>,
    );

    expect(await screen.findByText("推荐用途")).toBeInTheDocument();
    expect(screen.getByText("适合禁止出现儿童、孩童、kids、children 这类面向儿童用户的表达。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /knives/i }));

    await waitFor(() => {
      expect(screen.getByText("适合禁止出现刀具、刀刃、knife、blade 这类锐器或刀具表达。")).toBeInTheDocument();
    });
  });
});
