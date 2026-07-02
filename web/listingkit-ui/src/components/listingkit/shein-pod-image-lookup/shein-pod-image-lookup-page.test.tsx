import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinPODImageLookupPage } from "@/components/listingkit/shein-pod-image-lookup/shein-pod-image-lookup-page";
import * as api from "@/lib/api/shein-pod-image-lookup";

describe("SheinPODImageLookupPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("searches by store and seller SKU and renders source images", async () => {
    const lookup = vi.spyOn(api, "lookupSheinPODImages").mockResolvedValue({
      total: 1,
      items: [
        {
          task_id: "000a11f9-b41e-4e7f-bd9d-b3cefd739012",
          store_id: 869,
          seller_sku: "XB0606012001-V49720-T000A11F9-R4012C1-14624330",
          supplier_code: "XB0606012001-EEC0A584",
          shein_spu_name: "g2605302354951131",
          shein_version: "SPMP260530352497648",
          ai_original_image_url:
            "https://oss.shuomiai.com/listingkit-assets/20260530/d669b6d0-833c-4567-a39f-480e03a58fc3.png",
          sds_main_image_url:
            "https://cdn.sdspod.com/out/0/202605/f95d77f558fa121c28ba51b1f1926f5d.jpg",
          sds_gallery_image_urls: [
            "https://cdn.sdspod.com/out/36811/202605/1e49f4fd53b0807f99fbf58f9dae0e20.jpg",
          ],
          product_name: "Graphic Print Cosmetic Bag",
          prompt: "朋克叛逆人人格标签",
          status: "completed",
          created_at: "2026-05-30T22:52:54+08:00",
          updated_at: "2026-05-30T23:54:19+08:00",
        },
      ],
    });

    const user = userEvent.setup();
    render(<SheinPODImageLookupPage />);

    await user.clear(screen.getByLabelText("店铺 ID"));
    await user.type(screen.getByLabelText("店铺 ID"), "869");
    await user.type(
      screen.getByLabelText("查询关键词"),
      "XB0606012001V49720-T000A11F9-R4012C1-14624330",
    );
    await user.click(screen.getByRole("button", { name: "查询" }));

    await waitFor(() => {
      expect(lookup).toHaveBeenCalledWith(869, {
        query: "XB0606012001V49720-T000A11F9-R4012C1-14624330",
        limit: 20,
      });
    });
    expect(screen.getByText("g2605302354951131")).toBeInTheDocument();
    expect(screen.getByText("SPMP260530352497648")).toBeInTheDocument();
    expect(
      screen.getByText("XB0606012001-V49720-T000A11F9-R4012C1-14624330"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "AI 原图" })).toHaveAttribute(
      "href",
      "https://oss.shuomiai.com/listingkit-assets/20260530/d669b6d0-833c-4567-a39f-480e03a58fc3.png",
    );
    expect(screen.getByRole("link", { name: "任务" })).toHaveAttribute(
      "href",
      "/listing-kits/000a11f9-b41e-4e7f-bd9d-b3cefd739012/workspace?platform=shein",
    );
    expect(screen.getByAltText("AI 原图")).toHaveAttribute(
      "src",
      "https://oss.shuomiai.com/listingkit-assets/20260530/d669b6d0-833c-4567-a39f-480e03a58fc3.png",
    );
  });
});
