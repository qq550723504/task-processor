import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ProductWorkspaceCanonicalSection } from "@/components/listingkit/workspace/product-workspace-canonical-section";
import type { CanonicalProduct } from "@/lib/types/listingkit";

const product: CanonicalProduct = {
  title: "Canvas Tote",
  brand: "Demo Brand",
  category_path: ["Bags", "Totes"],
  description: "Reusable canvas tote for daily use.",
  images: [
    {
      url: "https://cdn.example.com/tote-front.jpg",
      alt: "Canvas Tote front",
      role: "main",
    },
  ],
  attributes: {
    material: { value: "Cotton" },
    weight: { value: "320", unit: "g" },
  },
  specifications: {
    technical: {
      closure: "Zip",
    },
    dimensions: {
      width: 38,
      height: 42,
    },
  },
  variants: [
    {
      sku: "TOTE-BLK",
      title: "Black",
      stock: 12,
    },
  ],
};

describe("ProductWorkspaceCanonicalSection", () => {
  it("shows canonical images in the product work area", () => {
    render(<ProductWorkspaceCanonicalSection product={product} section="images" />);

    expect(screen.getByRole("heading", { name: "图片" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Canvas Tote front" })).toHaveAttribute(
      "src",
      "https://cdn.example.com/tote-front.jpg",
    );
    expect(screen.getByText("main")).toBeInTheDocument();
  });

  it("shows canonical basic product facts", () => {
    render(<ProductWorkspaceCanonicalSection product={product} section="basic" />);

    expect(screen.getByRole("heading", { name: "基础信息" })).toBeInTheDocument();
    expect(screen.getByText("Canvas Tote")).toBeInTheDocument();
    expect(screen.getByText("Demo Brand")).toBeInTheDocument();
    expect(screen.getByText("Bags / Totes")).toBeInTheDocument();
  });

  it("shows canonical sku rows", () => {
    render(<ProductWorkspaceCanonicalSection product={product} section="sku" />);

    expect(screen.getByRole("heading", { name: "SKU" })).toBeInTheDocument();
    expect(screen.getByText("TOTE-BLK")).toBeInTheDocument();
    expect(screen.getByText("Black")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
  });

  it("shows specifications and attributes without exposing raw json", () => {
    const { rerender } = render(
      <ProductWorkspaceCanonicalSection product={product} section="specs" />,
    );

    expect(screen.getByRole("heading", { name: "规格" })).toBeInTheDocument();
    expect(screen.getByText("closure")).toBeInTheDocument();
    expect(screen.getByText("Zip")).toBeInTheDocument();
    expect(screen.queryByText(/\{"/)).not.toBeInTheDocument();

    rerender(<ProductWorkspaceCanonicalSection product={product} section="attributes" />);
    expect(screen.getByRole("heading", { name: "属性" })).toBeInTheDocument();
    expect(screen.getByText("material")).toBeInTheDocument();
    expect(screen.getByText("Cotton")).toBeInTheDocument();
    expect(screen.getByText("320 g")).toBeInTheDocument();
  });

  it("shows the canonical description and an explicit empty state", () => {
    const { rerender } = render(
      <ProductWorkspaceCanonicalSection product={product} section="description" />,
    );

    expect(screen.getByRole("heading", { name: "描述" })).toBeInTheDocument();
    expect(screen.getByText("Reusable canvas tote for daily use.")).toBeInTheDocument();

    rerender(<ProductWorkspaceCanonicalSection section="images" />);
    expect(screen.getByText("暂无商品图片")).toBeInTheDocument();
    expect(screen.queryByText(/Task|Temporal|Queue|任务 ID/i)).not.toBeInTheDocument();
  });
});
