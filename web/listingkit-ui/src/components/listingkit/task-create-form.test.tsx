import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { loadTaskCreateDraft } from "@/components/listingkit/task-create-draft";
import { TaskCreateForm } from "@/components/listingkit/task-create-form";

const push = vi.fn();
const mutateAsync = vi.fn();
const uploadMutateAsync = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push,
  }),
}));

vi.mock("@/lib/query/use-create-task", () => ({
  useCreateTask: () => ({
    mutateAsync,
    isPending: false,
  }),
}));

vi.mock("@/lib/query/use-upload-images", () => ({
  useUploadImages: () => ({
    mutateAsync: uploadMutateAsync,
    isPending: false,
    error: null,
  }),
}));

describe("TaskCreateForm", () => {
  beforeEach(() => {
    push.mockReset();
    mutateAsync.mockReset();
    uploadMutateAsync.mockReset();
    window.sessionStorage.clear();
  });

  it("submits the minimal request and routes to status", async () => {
    mutateAsync.mockResolvedValue({
      task_id: "task_123",
      status: "pending",
      created_at: "2026-04-19T00:00:00Z",
    });

    render(<TaskCreateForm />);

    fireEvent.change(screen.getByLabelText("Product title"), {
      target: { value: "Women knit cardigan" },
    });
    fireEvent.change(screen.getByLabelText("Image URLs"), {
      target: {
        value:
          "https://example.com/1.jpg\nhttps://example.com/2.jpg",
      },
    });
    fireEvent.click(screen.getByLabelText("Shein"));

    fireEvent.click(screen.getByRole("button", { name: "Create task" }));

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith({
        text: "Women knit cardigan",
        image_urls: ["https://example.com/1.jpg", "https://example.com/2.jpg"],
        platforms: ["shein"],
      });
    });

    expect(push).toHaveBeenCalledWith("/listing-kits/task_123/status");
    expect(loadTaskCreateDraft("task_123")).toEqual({
      text: "Women knit cardigan",
      imageUrls: "https://example.com/1.jpg\nhttps://example.com/2.jpg",
      productUrl: "",
      platforms: ["shein"],
      sceneCategory: "",
      sceneStyle: "",
      backgroundTone: "",
      composition: "",
      propsLevel: "",
      audienceHint: "",
      customSceneHint: "",
    });
  });

  it("allows product URL only creation", async () => {
    mutateAsync.mockResolvedValue({
      task_id: "task_456",
      status: "pending",
      created_at: "2026-04-19T00:00:00Z",
    });

    render(<TaskCreateForm />);

    fireEvent.click(screen.getByRole("tab", { name: "1688 / Product URL" }));
    expect(screen.getByText("Optional title")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Not required when you provide a product URL. Add it only if you want to override or improve the listing title.",
      ),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Product URL"), {
      target: { value: "https://detail.1688.com/offer/123456789.html" },
    });
    fireEvent.click(screen.getByLabelText("Temu"));

    fireEvent.click(screen.getByRole("button", { name: "Create task" }));

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith({
        text: "",
        image_urls: [],
        product_url: "https://detail.1688.com/offer/123456789.html",
        platforms: ["temu"],
      });
    });

    expect(push).toHaveBeenCalledWith("/listing-kits/task_456/status");
    expect(loadTaskCreateDraft("task_456")).toEqual({
      text: "",
      imageUrls: "",
      productUrl: "https://detail.1688.com/offer/123456789.html",
      platforms: ["temu"],
      sceneCategory: "",
      sceneStyle: "",
      backgroundTone: "",
      composition: "",
      propsLevel: "",
      audienceHint: "",
      customSceneHint: "",
    });
  });

  it("shows input quality guidance for image count and text length", () => {
    render(<TaskCreateForm />);

    expect(screen.getByText("Product title")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Recommended for image-driven creation. Stronger title text helps ListingKit pass the current quality gate.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Start with a product title, image URLs, or a product URL such as a 1688 listing, then choose the target platforms.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Use public image URLs, upload local image files, or paste a product URL such as a 1688 listing.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Input quality guidance")).toBeInTheDocument();
    expect(screen.getByText("Need at least 3 image URLs")).toBeInTheDocument();
    expect(screen.getByText("Need at least 50 characters of product text")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Product title"), {
      target: { value: "Women knit cardigan with textured fabric and soft handfeel." },
    });
    fireEvent.change(screen.getByLabelText("Image URLs"), {
      target: {
        value:
          "https://example.com/1.jpg\nhttps://example.com/2.jpg\nhttps://example.com/3.jpg",
      },
    });

    expect(screen.getByText("3 / 3 ready")).toBeInTheDocument();
    expect(screen.getByText("59 / 50 ready")).toBeInTheDocument();
  });

  it("prefills values from an existing draft", () => {
    render(
      <TaskCreateForm
        initialValues={{
          text: "Improved cardigan title",
          imageUrls: "https://example.com/a.jpg\nhttps://example.com/b.jpg",
          productUrl: "https://detail.1688.com/offer/123456789.html",
          platforms: ["temu"],
        }}
      />,
    );

    expect(screen.getByLabelText("Product title")).toHaveValue(
      "Improved cardigan title",
    );
    expect(screen.getByRole("tab", { name: "Image URLs" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
    expect(screen.getByLabelText("Product URL")).toHaveValue(
      "https://detail.1688.com/offer/123456789.html",
    );
    fireEvent.click(screen.getByRole("tab", { name: "Image URLs" }));
    expect(screen.getByLabelText("Image URLs")).toHaveValue(
      "https://example.com/a.jpg\nhttps://example.com/b.jpg",
    );
    expect(screen.getByLabelText("Temu")).toBeChecked();
  });

  it("focuses the requested field when reopening an improved task", () => {
    render(
      <TaskCreateForm
        initialValues={{
          text: "Improved cardigan title",
          imageUrls: "https://example.com/a.jpg",
          productUrl: "",
          platforms: ["temu"],
        }}
        initialFocus="imageUrls"
        fieldIssues={["imageUrls", "text"]}
      />,
    );

    expect(screen.getByLabelText("Image URLs")).toHaveFocus();
    expect(
      screen.getByText("The previous task failed on image coverage. Add at least 3 strong product images."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("The previous task failed on product copy quality. Expand the title or description input."),
    ).toBeInTheDocument();
  });

  it("lets the user jump directly to URL or image source fields", async () => {
    render(<TaskCreateForm />);

    fireEvent.click(screen.getByRole("tab", { name: "1688 / Product URL" }));
    await waitFor(() => {
      expect(screen.getByLabelText("Product URL")).toHaveFocus();
    });

    fireEvent.click(screen.getByRole("tab", { name: "Image URLs" }));
    await waitFor(() => {
      expect(screen.getByLabelText("Image URLs")).toHaveFocus();
    });
  });

  it("blocks submit when no source input is provided", async () => {
    render(<TaskCreateForm />);

    fireEvent.click(screen.getByLabelText("Shein"));
    fireEvent.click(screen.getByRole("button", { name: "Create task" }));

    await waitFor(() => {
      expect(
        screen.getByText("Provide at least one of product title, image URLs, or product URL."),
      ).toBeInTheDocument();
    });

    expect(mutateAsync).not.toHaveBeenCalled();
  });

  it("uploads local files and appends returned URLs", async () => {
    uploadMutateAsync.mockResolvedValue({
      image_urls: [
        "http://localhost:8080/api/v1/listing-kits/uploads/files/a.jpg",
        "http://localhost:8080/api/v1/listing-kits/uploads/files/b.jpg",
      ],
    });

    render(<TaskCreateForm />);

    const fileInput = screen.getByLabelText("Upload images");
    const firstFile = new File(["a"], "a.jpg", { type: "image/jpeg" });
    const secondFile = new File(["b"], "b.png", { type: "image/png" });
    fireEvent.change(fileInput, {
      target: { files: [firstFile, secondFile] },
    });

    fireEvent.click(screen.getByRole("button", { name: "Upload selected images" }));

    await waitFor(() => {
      expect(uploadMutateAsync).toHaveBeenCalledWith([firstFile, secondFile]);
    });

    expect(screen.getByLabelText("Image URLs")).toHaveValue(
      "http://localhost:8080/api/v1/listing-kits/uploads/files/a.jpg\nhttp://localhost:8080/api/v1/listing-kits/uploads/files/b.jpg",
    );
  });

  it("submits structured scene customization when provided", async () => {
    mutateAsync.mockResolvedValue({
      task_id: "task_scene_123",
      status: "pending",
      created_at: "2026-04-20T00:00:00Z",
    });

    render(<TaskCreateForm />);

    fireEvent.change(screen.getByLabelText("Product title"), {
      target: { value: "Red running sneaker" },
    });
    fireEvent.change(screen.getByLabelText("Image URLs"), {
      target: {
        value:
          "https://example.com/1.jpg\nhttps://example.com/2.jpg\nhttps://example.com/3.jpg",
      },
    });
    fireEvent.click(screen.getByLabelText("Amazon"));
    fireEvent.click(screen.getByRole("button", { name: "Customize scene generation" }));

    fireEvent.change(screen.getByLabelText("Scene category"), {
      target: { value: "shoes" },
    });
    fireEvent.change(screen.getByLabelText("Scene style"), {
      target: { value: "lifestyle" },
    });
    fireEvent.change(screen.getByLabelText("Background tone"), {
      target: { value: "warm" },
    });
    fireEvent.change(screen.getByLabelText("Composition"), {
      target: { value: "close_up" },
    });
    fireEvent.change(screen.getByLabelText("Props level"), {
      target: { value: "light" },
    });
    fireEvent.change(screen.getByLabelText("Audience hint"), {
      target: { value: "sporty" },
    });
    fireEvent.change(screen.getByLabelText("Custom scene hint"), {
      target: { value: "show subtle motion energy" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Create task" }));

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith({
        text: "Red running sneaker",
        image_urls: [
          "https://example.com/1.jpg",
          "https://example.com/2.jpg",
          "https://example.com/3.jpg",
        ],
        platforms: ["amazon"],
        options: {
          scene: {
            scene_category: "shoes",
            scene_style: "lifestyle",
            background_tone: "warm",
            composition: "close_up",
            props_level: "light",
            audience_hint: "sporty",
            custom_scene_hint: "show subtle motion energy",
          },
        },
      });
    });

    expect(loadTaskCreateDraft("task_scene_123")).toEqual({
      text: "Red running sneaker",
      imageUrls:
        "https://example.com/1.jpg\nhttps://example.com/2.jpg\nhttps://example.com/3.jpg",
      productUrl: "",
      platforms: ["amazon"],
      sceneCategory: "shoes",
      sceneStyle: "lifestyle",
      backgroundTone: "warm",
      composition: "close_up",
      propsLevel: "light",
      audienceHint: "sporty",
      customSceneHint: "show subtle motion energy",
    });
  });

  it("prefills structured scene customization from an existing draft", () => {
    render(
      <TaskCreateForm
        initialValues={{
          text: "Improved sneaker title",
          imageUrls: "https://example.com/a.jpg",
          productUrl: "",
          platforms: ["amazon"],
          sceneCategory: "shoes",
          sceneStyle: "lifestyle",
          backgroundTone: "warm",
          composition: "close_up",
          propsLevel: "light",
          audienceHint: "sporty",
          customSceneHint: "show subtle motion energy",
        }}
      />,
    );

    expect(screen.getByLabelText("Scene category")).toHaveValue("shoes");
    expect(screen.getByLabelText("Scene style")).toHaveValue("lifestyle");
    expect(screen.getByLabelText("Background tone")).toHaveValue("warm");
    expect(screen.getByLabelText("Composition")).toHaveValue("close_up");
    expect(screen.getByLabelText("Props level")).toHaveValue("light");
    expect(screen.getByLabelText("Audience hint")).toHaveValue("sporty");
    expect(screen.getByLabelText("Custom scene hint")).toHaveValue(
      "show subtle motion energy",
    );
  });
});
