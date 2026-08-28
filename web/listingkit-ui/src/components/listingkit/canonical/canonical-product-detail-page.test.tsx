import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CanonicalProductDetailPage } from "@/components/listingkit/canonical/canonical-product-detail-page";
import { ApiError } from "@/lib/api/api-error";

type DetailQueryMock = {
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  data: unknown;
};

const detailQuery = vi.hoisted(() => ({
  current: null as DetailQueryMock | null,
}));

const successDetail = {
  taskId: "task-1",
  sourceReference: {
    key: "crawler:1688:888",
    type: "crawler",
    platform: "1688",
    id: "888",
    url: "https://detail.1688.com/offer/888.html",
  },
  workspaceHref: "/listing-kits/task-1/workspace?platform=shein",
  product: {
    title: "Canvas Tote",
    brand: "Studio",
    category_path: ["Bags"],
    images: [
      { url: "https://example.com/main.jpg", role: "main" },
      { url: "https://example.com/detail.jpg", role: "gallery" },
      { url: "https://example.com/main.jpg", role: "duplicate" },
    ],
  },
  summary: {
    taskId: "task-1",
    title: "Canvas Tote",
    categoryPath: ["Bags"],
    imageUrl: "https://example.com/main.jpg",
    platformLabels: ["shein"],
    needsReview: false,
    imageCount: 3,
    variantCount: 0,
  },
  reviewFieldCount: 0,
  trustedFieldCount: 0,
  fieldTraces: [],
};

vi.mock("@/lib/query/use-canonical-products", () => ({
  useCanonicalProductDetail: () => detailQuery.current,
}));

describe("CanonicalProductDetailPage", () => {
  beforeEach(() => {
    detailQuery.current = {
      isLoading: false,
      isError: false,
      error: null,
      data: successDetail,
    };
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the full canonical image gallery instead of only the cover image", () => {
    render(<CanonicalProductDetailPage taskId="task-1" />);

    expect(screen.getByText("2 张")).toBeInTheDocument();
    expect(screen.getByText("main")).toBeInTheDocument();
    expect(screen.getByText("gallery")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Canvas Tote/i })).toHaveLength(2);
  });

  it("shows the persisted source lineage", () => {
    render(<CanonicalProductDetailPage taskId="task-1" />);

    expect(screen.getByText("来源 1688 · 888")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
      "href",
      "https://detail.1688.com/offer/888.html",
    );
  });

  it("uses product language and keeps execution identity secondary", () => {
    render(<CanonicalProductDetailPage taskId="task-1" />);

    expect(screen.getByRole("link", { name: "返回商品中心" })).toHaveAttribute(
      "href",
      "/listing-kits/canonical-products",
    );
    expect(screen.getByText("商品详情")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "编辑商品" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/workspace?platform=shein",
    );
    expect(screen.getByRole("link", { name: "查看执行记录" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/status",
    );
    expect(screen.getByText("需确认字段")).toBeInTheDocument();
    expect(screen.getByText("已校验")).toBeInTheDocument();
    expect(screen.getByText("字段依据")).toBeInTheDocument();
    expect(screen.queryByText("task-1")).not.toBeInTheDocument();
    expect(screen.queryByText(/canonical_product/i)).not.toBeInTheDocument();
  });

  it("preserves the existing workspace and execution routes", () => {
    render(<CanonicalProductDetailPage taskId="task-1" />);

    expect(screen.getByRole("link", { name: "查看执行记录" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/status",
    );
    expect(screen.getByRole("link", { name: "编辑商品" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/workspace?platform=shein",
    );
  });

  it("treats a successful task without a canonical product as missing", () => {
    detailQuery.current = {
      isLoading: false,
      isError: false,
      error: null,
      data: null,
    };

    render(<CanonicalProductDetailPage taskId="task-missing-product" />);

    expect(screen.getByText("未找到商品")).toBeInTheDocument();
    expect(
      screen.getByText("这份商品资料不存在或尚未生成，请返回商品中心重新选择。"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/稍后重试/)).not.toBeInTheDocument();
  });

  it("treats a 404 task response as a stale or missing product link", () => {
    detailQuery.current = {
      isLoading: false,
      isError: true,
      error: new ApiError("task not found", 404),
      data: undefined,
    };

    render(<CanonicalProductDetailPage taskId="task-404" />);

    expect(screen.getByText("未找到商品")).toBeInTheDocument();
    expect(
      screen.getByText("商品不存在或链接已失效，请返回商品中心重新选择。"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/稍后重试/)).not.toBeInTheDocument();
  });

  it("keeps retry guidance for transient product detail failures", () => {
    detailQuery.current = {
      isLoading: false,
      isError: true,
      error: new ApiError("temporary upstream failure", 500),
      data: undefined,
    };

    render(<CanonicalProductDetailPage taskId="task-500" />);

    expect(screen.getByText("商品加载失败")).toBeInTheDocument();
    expect(screen.getByText("暂时无法读取这份商品资料，请稍后重试。")).toBeInTheDocument();
  });
});
