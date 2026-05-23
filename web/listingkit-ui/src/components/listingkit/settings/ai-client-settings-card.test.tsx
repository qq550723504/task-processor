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
    mocks.useAIClientSettings.mockImplementation((_scope: string, clientName: string) => ({
      data: {
        scope: "tenant",
        client_name: clientName,
        resolved_scope: "tenant",
        api_key_set: clientName !== "image_gpt_image_2",
        base_url:
          clientName === "image_nanobanana"
            ? "https://tenant-nano.example.com/v1"
            : clientName === "image_gpt_image_2"
            ? "https://tenant-image.example.com/v1"
            : "https://tenant-ai.example.com/v1",
        model:
          clientName === "image_nanobanana"
            ? "nano-banana-fast"
            : clientName === "image_gpt_image_2"
              ? "gpt-image-2"
              : "gpt-4.1-mini",
        enabled: true,
      },
      isLoading: false,
      isError: false,
    }));
    mocks.useUpdateAIClientSettings.mockReturnValue({
      mutate: mocks.mutate,
      isPending: false,
      error: null,
    });
  });

  it("shows configured key status without exposing the raw api key", () => {
    render(<AIClientSettingsCard />);

    expect(screen.getByText("密钥已配置")).toBeInTheDocument();
    expect(screen.getByText("当前生效来源：")).toBeInTheDocument();
    expect(screen.getByText("当前租户配置")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("tenant-secret")).not.toBeInTheDocument();
    expect(screen.getByDisplayValue("https://tenant-ai.example.com/v1")).toBeInTheDocument();
    expect(screen.getByDisplayValue("gpt-4.1-mini")).toBeInTheDocument();
  });

  it("saves tenant scoped endpoint and model settings", () => {
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
    fireEvent.click(screen.getByRole("button", { name: "保存 通用文案 配置" }));

    expect(mocks.mutate).toHaveBeenCalledWith({
      scope: "tenant",
      client_name: "default",
      api_key: "new-secret",
      base_url: "https://new-endpoint.example.com/v1",
      model: "gpt-4.1",
      enabled: true,
    });
  });

  it("saves current-user scoped settings without a custom user id field", () => {
    render(<AIClientSettingsCard />);

    fireEvent.change(screen.getByLabelText("配置范围"), {
      target: { value: "user" },
    });
    fireEvent.change(screen.getByLabelText("API Key"), {
      target: { value: "user-secret" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存 通用文案 配置" }));

    expect(mocks.mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        scope: "user",
        api_key: "user-secret",
      }),
    );
    expect(screen.queryByLabelText("User ID")).not.toBeInTheDocument();
  });

  it("switches to Nano Banana settings and saves with image client name", () => {
    render(<AIClientSettingsCard />);

    fireEvent.click(screen.getByRole("button", { name: /Nano Banana/ }));

    expect(screen.getByDisplayValue("https://tenant-nano.example.com/v1")).toBeInTheDocument();
    expect(screen.getByDisplayValue("nano-banana-fast")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Model"), {
      target: { value: "nano-banana-pro" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存 Nano Banana 配置" }));

    expect(mocks.mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        client_name: "image_nanobanana",
        model: "nano-banana-pro",
      }),
    );
  });

  it("shows user scope when a user-level config is currently taking effect", () => {
    mocks.useAIClientSettings.mockImplementation((_scope: string, clientName: string) => ({
      data: {
        scope: "user",
        client_name: clientName,
        resolved_scope: "user",
        api_key_set: true,
        base_url: "https://user-ai.example.com/v1",
        model: "gemini-3.1-flash-lite",
        enabled: true,
      },
      isLoading: false,
      isError: false,
    }));

    render(<AIClientSettingsCard />);

    expect(screen.getByText("当前登录用户配置")).toBeInTheDocument();
  });
});
