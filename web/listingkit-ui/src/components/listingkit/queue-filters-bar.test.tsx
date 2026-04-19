import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { QueueFiltersBar } from "@/components/listingkit/queue-filters-bar";

describe("QueueFiltersBar", () => {
  it("submits updated filter values", async () => {
    const user = userEvent.setup();
    const handleApply = vi.fn();

    render(
      <QueueFiltersBar
        value={{
          platform: "",
          slot: "",
          quality_grade: "",
          preview_capability: "",
          review_status: "",
          render_preview_available: false,
        }}
        onApply={handleApply}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Platform"), "shein");
    await user.selectOptions(
      screen.getByLabelText("Preview Capability"),
      "detail_preview",
    );
    await user.click(screen.getByLabelText("Has Preview"));
    await user.click(screen.getByRole("button", { name: "Apply Filters" }));

    expect(handleApply).toHaveBeenCalledWith(
      expect.objectContaining({
        platform: "shein",
        preview_capability: "detail_preview",
        render_preview_available: true,
      }),
    );
  }, 15000);
});
