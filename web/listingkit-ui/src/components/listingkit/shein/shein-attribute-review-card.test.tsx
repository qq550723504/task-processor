import { render, screen } from "@testing-library/react";

import { SheinAttributeReviewCard } from "@/components/listingkit/shein/shein-attribute-review-card";

describe("SheinAttributeReviewCard", () => {
  it("renders attribute mapping summary and notes", () => {
    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              resolved_count: 3,
              unresolved_count: 1,
              resolved_attributes: [
                {
                  name: "Upper Material",
                  value: "flyknit",
                  attribute_id: 112,
                  attribute_value_id: 5930427,
                },
              ],
              review_notes: ["还有 1 个普通属性未命中模板值"],
            },
          },
        }}
      />,
    );

    expect(screen.getByText("SHEIN attribute review")).toBeInTheDocument();
    expect(screen.getByText("Upper Material")).toBeInTheDocument();
    expect(screen.getByText("flyknit")).toBeInTheDocument();
    expect(screen.getByText("attribute_id 112 · value_id 5930427")).toBeInTheDocument();
    expect(screen.getByText("还有 1 个普通属性未命中模板值")).toBeInTheDocument();
  });
});
