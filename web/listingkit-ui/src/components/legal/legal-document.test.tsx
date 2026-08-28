import { render, screen } from "@testing-library/react";

import { LegalDocument } from "./legal-document";

describe("LegalDocument", () => {
  it("publishes the confirmed operator and contact details without an internal launch reminder", () => {
    render(
      <LegalDocument
        title="隐私政策"
        summary="说明个人信息处理规则。"
        sections={[{ heading: "一、处理规则", paragraphs: ["仅在必要范围内处理信息。"] }]}
      />,
    );

    expect(screen.getByText("武汉市硕米科技有限公司")).toBeInTheDocument();
    expect(screen.getByText("武汉市洪山区吴家湾大厦 1808")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "support@shuomiai.com" })).toHaveAttribute(
      "href",
      "mailto:support@shuomiai.com",
    );
    expect(screen.getByText(/中华人民共和国法律/)).toBeInTheDocument();
    expect(screen.queryByText(/正式上线前/)).not.toBeInTheDocument();
  });
});
