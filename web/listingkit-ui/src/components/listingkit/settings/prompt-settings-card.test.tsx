import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PromptSettingsCard } from "@/components/listingkit/settings/prompt-settings-card";

const upsertMock = vi.fn();
const statusMock = vi.fn();
const promptQueryState = {
  isError: false,
};
const upsertState = {
  error: null as Error | null,
};

vi.mock("@/lib/query/use-prompt-settings", () => ({
  usePromptSettings: () => ({
    data: {
      items: [
        {
          key: "shein.content_optimizer.optimize_title_description_system",
          content: "System prompt",
          version: "v1",
          enabled: true,
        },
      ],
    },
    isLoading: false,
    isError: promptQueryState.isError,
  }),
  useUpsertPromptSetting: () => ({
    mutate: upsertMock,
    isPending: false,
    error: upsertState.error,
  }),
  useSetPromptSettingStatus: () => ({
    mutate: statusMock,
    isPending: false,
    error: null,
  }),
}));

describe("PromptSettingsCard", () => {
  beforeEach(() => {
    promptQueryState.isError = false;
    upsertState.error = null;
    upsertMock.mockClear();
    statusMock.mockClear();
  });

  it("loads a prompt into the editor and saves changes", () => {
    render(<PromptSettingsCard />);

    fireEvent.click(
      screen.getByRole("button", {
        name: /shein.content_optimizer.optimize_title_description_system/,
      }),
    );
    fireEvent.change(screen.getByLabelText("Prompt 内容"), {
      target: { value: "Updated prompt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存提示词" }));

    expect(upsertMock).toHaveBeenCalledWith(
      expect.objectContaining({
        key: "shein.content_optimizer.optimize_title_description_system",
        content: "Updated prompt",
        version: "v1",
        enabled: true,
      }),
    );
  });

  it("renders prompt failures as alerts", () => {
    promptQueryState.isError = true;
    upsertState.error = new Error("save failed");

    render(<PromptSettingsCard />);

    expect(screen.getAllByRole("status")).toHaveLength(2);
  });
});
