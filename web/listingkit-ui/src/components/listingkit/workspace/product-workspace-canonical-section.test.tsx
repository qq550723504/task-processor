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
      unit: "cm",
    },
    package: {
      dimensions: {
        width: 40,
        height: 44,
        unit: "cm",
      },
      quantity: 1,
    },
  },
  variants: [
    {
      sku: "TOTE-BLK-M",
      price: { amount: 29.9, currency: "CNY" },
      attributes: {
        Color: { value: "Black" },
        Size: { value: "M" },
      },
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

  it("shows variant-only images with their SKU context", () => {
    render(
      <ProductWorkspaceCanonicalSection
        product={{
          title: "Variant Tote",
          variants: [
            {
              sku: "TOTE-RED",
              images: [
                {
                  url: "https://cdn.example.com/tote-red.jpg",
                  role: "swatch",
                },
              ],
            },
          ],
        }}
        section="images"
      />,
    );

    expect(screen.queryByText("暂无商品图片")).not.toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Variant Tote - TOTE-RED" })).toHaveAttribute(
      "src",
      "https://cdn.example.com/tote-red.jpg",
    );
    expect(screen.getByText("TOTE-RED · swatch")).toBeInTheDocument();
  });

  it("shows canonical basic product facts", () => {
    render(<ProductWorkspaceCanonicalSection product={product} section="basic" />);

    expect(screen.getByRole("heading", { name: "基础信息" })).toBeInTheDocument();
    expect(screen.getByText("Canvas Tote")).toBeInTheDocument();
    expect(screen.getByText("Demo Brand")).toBeInTheDocument();
    expect(screen.getByText("Bags / Totes")).toBeInTheDocument();
  });

  it("shows the missing-title fallback when the canonical title is whitespace only", () => {
    render(
      <ProductWorkspaceCanonicalSection
        product={{ ...product, title: "   " }}
        section="basic"
      />,
    );

    expect(screen.getByText("暂无标题")).toBeInTheDocument();
  });

  it("shows canonical selling points and SEO keywords in basic information", () => {
    render(
      <ProductWorkspaceCanonicalSection
        product={{
          ...product,
          selling_points: ["Foldable", "Water resistant"],
          seo_keywords: ["canvas tote", "reusable bag"],
        }}
        section="basic"
      />,
    );

    expect(screen.getByText("商品卖点")).toBeInTheDocument();
    expect(screen.getByText("Foldable · Water resistant")).toBeInTheDocument();
    expect(screen.getByText("SEO 关键词")).toBeInTheDocument();
    expect(screen.getByText("canvas tote · reusable bag")).toBeInTheDocument();
  });

  it("shows backend-shaped canonical sku attributes and treats omitted stock as zero", () => {
    render(<ProductWorkspaceCanonicalSection product={product} section="sku" />);

    expect(screen.getByRole("heading", { name: "SKU" })).toBeInTheDocument();
    expect(screen.getByText("TOTE-BLK-M")).toBeInTheDocument();
    expect(screen.getByText("Color: Black · Size: M")).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: /CNY.*29\.90/ })).toBeInTheDocument();
    expect(screen.getByText("0")).toBeInTheDocument();
  });

  it("shows the per-variant cost price separately from the selling price", () => {
    render(
      <ProductWorkspaceCanonicalSection
        product={{
          ...product,
          variants: [
            {
              ...product.variants?.[0],
              price: { amount: 29.9, cost_price: 12.5, currency: "CNY" },
            },
          ],
        }}
        section="sku"
      />,
    );

    const priceCell = screen.getByRole("cell", { name: /售价.*CNY.*29\.90/ });
    expect(priceCell).toHaveTextContent("售价：CNY 29.90");
    expect(priceCell).toHaveTextContent("成本价：CNY 12.50");
  });

  it("shows canonical barcode and default variant metadata", () => {
    render(
      <ProductWorkspaceCanonicalSection
        product={{
          ...product,
          variants: [
            {
              ...product.variants?.[0],
              barcode: "6901234567890",
              is_default: true,
            },
          ],
        }}
        section="sku"
      />,
    );

    expect(screen.getByRole("columnheader", { name: "条码" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "默认" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "6901234567890" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "是" })).toBeInTheDocument();
  });

  it("shows per-variant dimensions and weight", () => {
    render(
      <ProductWorkspaceCanonicalSection
        product={{
          ...product,
          variants: [
            {
              ...product.variants?.[0],
              dimensions: { length: 10, width: 20, height: 30, unit: "cm" },
              weight: { value: 500, unit: "g" },
            },
          ],
        }}
        section="sku"
      />,
    );

    expect(screen.getByRole("columnheader", { name: "尺寸" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "重量" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "10 × 20 × 30 cm" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "500 g" })).toBeInTheDocument();
  });

  it("preserves product and package specification context without exposing raw json", () => {
    const { rerender } = render(
      <ProductWorkspaceCanonicalSection product={product} section="specs" />,
    );

    expect(screen.getByRole("heading", { name: "规格" })).toBeInTheDocument();
    expect(screen.getByText("closure")).toBeInTheDocument();
    expect(screen.getByText("Zip")).toBeInTheDocument();
    expect(screen.getByText("商品尺寸 · width")).toBeInTheDocument();
    expect(screen.getByText("包装尺寸 · width")).toBeInTheDocument();
    expect(screen.queryByText(/\{"/)).not.toBeInTheDocument();

    rerender(<ProductWorkspaceCanonicalSection product={product} section="attributes" />);
    expect(screen.getByRole("heading", { name: "属性" })).toBeInTheDocument();
    expect(screen.getByText("material")).toBeInTheDocument();
    expect(screen.getByText("Cotton")).toBeInTheDocument();
    expect(screen.getByText("320 g")).toBeInTheDocument();
  });

  it("shows canonical variation options when physical specifications are absent", () => {
    render(
      <ProductWorkspaceCanonicalSection
        product={{
          ...product,
          specifications: undefined,
          variant_dimensions: [
            { name: "Color", values: ["Black", "Red"] },
            { name: "Size", values: ["M", "L"] },
          ],
        } as CanonicalProduct}
        section="specs"
      />,
    );

    expect(screen.getByText("Color")).toBeInTheDocument();
    expect(screen.getByText("Black · Red")).toBeInTheDocument();
    expect(screen.getByText("Size")).toBeInTheDocument();
    expect(screen.getByText("M · L")).toBeInTheDocument();
    expect(screen.queryByText("暂无规格信息")).not.toBeInTheDocument();
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

  it("shows the missing-description fallback when the canonical description is whitespace only", () => {
    render(
      <ProductWorkspaceCanonicalSection
        product={{ ...product, description: "   " }}
        section="description"
      />,
    );

    expect(screen.getByText("暂无商品描述")).toBeInTheDocument();
  });
});
