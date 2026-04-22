import { render, screen } from "@testing-library/react";

import { SheinSaleAttributeReviewCard } from "@/components/listingkit/shein-sale-attribute-review-card";

describe("SheinSaleAttributeReviewCard", () => {
  it("renders current sale attribute summary and candidate reasons", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 27,
              selection_summary: ["主销售属性已命中，次销售属性仍需确认"],
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Color",
                  value: "Black",
                  attribute_id: 27,
                  attribute_value_id: 112,
                },
              ],
              candidates: [
                {
                  source_dimension: "颜色",
                  name: "Color",
                  selected_scope: "primary",
                  reasons: ["模板值拟合度 1/2"],
                },
              ],
              review_notes: ["尺码模板候选仍待确认"],
            },
          },
        }}
      />,
    );

    expect(screen.getByText("SHEIN sale attribute review")).toBeInTheDocument();
    expect(screen.getAllByText("Color")).toHaveLength(2);
    expect(screen.getByText("Black")).toBeInTheDocument();
    expect(screen.getByText("attribute_id 27 · value_id 112")).toBeInTheDocument();
    expect(screen.getByText("主销售属性已命中，次销售属性仍需确认")).toBeInTheDocument();
    expect(screen.getByText("模板值拟合度 1/2")).toBeInTheDocument();
    expect(screen.getByText("尺码模板候选仍待确认")).toBeInTheDocument();
  });
});
