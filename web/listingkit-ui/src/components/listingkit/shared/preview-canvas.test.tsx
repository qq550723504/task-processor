import { render, screen } from "@testing-library/react";

import { PreviewCanvas } from "@/components/listingkit/shared/preview-canvas";

describe("PreviewCanvas", () => {
  it("renders raster image previews when svg sidecar is unavailable", () => {
    render(
      <PreviewCanvas
        preview={{
          asset_id: "gallery-rendered-1",
          asset_url:
            "http://127.0.0.1:9100/listingkit-assets/gallery-rendered-1.png",
          template_label: "SHEIN Lifestyle Gallery",
        }}
      />,
    );

    const image = screen.getByRole("img", {
      name: "SHEIN Lifestyle Gallery",
    });
    expect(image).toHaveAttribute(
      "src",
      "http://127.0.0.1:9100/listingkit-assets/gallery-rendered-1.png",
    );
  });
});
