import { fireEvent, render, screen } from "@testing-library/react";

import { AIClientSettingsCard } from "@/components/listingkit/settings/ai-client-settings-card";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  useAIClientSettings: vi.fn(),
  useUpdateAIClientSettings: vi.fn(),
}));

vi.mock("@/lib/query/use-ai-client-settings", () => ({
  useAIClientSettings: (scope: string, clientName: string) =>
    mocks.useAIClientSettings(scope, clientName),
  useUpdateAIClientSettings: () => mocks.useUpdateAIClientSettings(),
}));

describe("AIClientSettingsCard", () => {
  beforeEach(() => {
    mocks.mutate.mockReset();
    mocks.useAIClientSettings.mockReset();
    mocks.useUpdateAIClientSettings.mockReset();
    mocks.useAIClientSettings.mockReturnValue({
      data: {
        scope: "tenant",
        client_name: "default",
        api_key_set: true,
        base_url: "https://tenant-ai.example.com/v1",
        model: "gpt-4.1-mini",
        timeout_second: 45,
        enabled: true,
      },
      isLoading: false,
      isError: false,
    });
    mocks.useUpdateAIClientSettings.mockReturnValue({
      mutate: mocks.mutate,
      isPending: false,
      error: null,
    });
  });

  it("shows configured key status without exposing the raw api key", () => {
    render(<AIClientSettingsCard />);

    expect(screen.getByText("密钥已配置")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("tenant-secret")).not.toBeInTheDocument();
    expect(screen.getByDisplayValue("https://tenant-ai.example.com/v1")).toBeInTheDocument();
    expect(screen.getByDisplayValue("gpt-4.1-mini")).toBeInTheDocument();
  });

  it("saves customer managed endpoint and model settings", () => {
    render(<AIClientSettingsCard />);

    fireEvent.change(screen.getByLabelText("API Key"), {
      target: { value: "new-secret" },
    });
    fireEvent.change(screen.getByLabelText("Endpoint"), {
      target: { value: "https://new-endpoint.example.com/v1" },
    });
    fireEvent.change(screen.getByLabelText("Model"), {
      target: { value: "gpt-4.1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存 AI 配置" }));

    expect(mocks.mutate).toHaveBeenCalledWith({
      scope: "tenant",
      client_name: "default",
      api_key: "new-secret",
      base_url: "https://new-endpoint.example.com/v1",
      model: "gpt-4.1",
      timeout_second: 45,
      enabled: true,
    });
  });

  it("passes user id when saving user scoped settings", () => {
    render(<AIClientSettingsCard />);

    fireEvent.change(screen.getByLabelText("配置范围"), {
      target: { value: "user" },
    });
    fireEvent.change(screen.getByLabelText("User ID"), {
      target: { value: "user-1" },
    });
    fireEvent.change(screen.getByLabelText("API Key"), {
      target: { value: "user-secret" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存 AI 配置" }));

    expect(mocks.mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        scope: "user",
        user_id: "user-1",
        api_key: "user-secret",
      }),
    );
  });
});
