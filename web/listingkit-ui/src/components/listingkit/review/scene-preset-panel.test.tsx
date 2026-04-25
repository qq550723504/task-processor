import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ScenePresetPanel } from "@/components/listingkit/review/scene-preset-panel";

describe("ScenePresetPanel", () => {
  it("renders scene preset fields when summary is present", () => {
    render(
      <ScenePresetPanel
        summary={{
          prompt_key: "productimage.scene.shoes",
          defaults_source: "platform_category",
          scene_category: "shoes",
          scene_style: "studio",
          background_tone: "bright",
          composition: "centered",
          props_level: "none",
          audience_hint: "premium",
          custom_scene_hint: "Keep shadows soft.",
        }}
      />,
    );

    expect(screen.getByText("Scene preset")).toBeInTheDocument();
    expect(screen.getByText("productimage.scene.shoes")).toBeInTheDocument();
    expect(
      screen.getByText("Platform + category default"),
    ).toBeInTheDocument();
    expect(screen.getByText("Shoes")).toBeInTheDocument();
    expect(screen.getByText("Studio")).toBeInTheDocument();
    expect(screen.getByText("Keep shadows soft.")).toBeInTheDocument();
  });

  it("renders nothing when summary is empty", () => {
    const { container } = render(<ScenePresetPanel summary={{}} />);

    expect(container).toBeEmptyDOMElement();
  });
});
