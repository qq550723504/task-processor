import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { loadTaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";
import { TaskCreateForm } from "@/components/listingkit/tasks/task-create-form";

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

  it("defaults advanced settings to collapsed and reveals them on demand", () => {
    render(<TaskCreateForm />);

    expect(screen.queryByText("SDS 同步设置")).not.toBeInTheDocument();
    expect(screen.queryByText("场景生成设置")).not.toBeInTheDocument();
    expect(
      screen.getByText("先填写基础信息；SDS 和场景等高级配置可以稍后再补充。"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "显示高级设置" }));

    expect(screen.getByText("SDS 同步设置")).toBeInTheDocument();
    expect(screen.getByText("场景生成设置")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "收起高级设置" })).toBeInTheDocument();
  });

  it("submits the minimal request and routes to status", async () => {
    mutateAsync.mockResolvedValue({
      task_id: "task_123",
      status: "pending",
      created_at: "2026-04-19T00:00:00Z",
    });

    render(<TaskCreateForm />);

    fireEvent.change(screen.getByLabelText("商品标题"), {
      target: { value: "Women knit cardigan" },
    });
    fireEvent.change(screen.getByLabelText("图片链接"), {
      target: {
        value: "https://example.com/1.jpg\nhttps://example.com/2.jpg",
      },
    });
    fireEvent.click(screen.getByLabelText("SHEIN"));

    fireEvent.click(screen.getByRole("button", { name: "创建任务" }));

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
      sheinStoreId: "",
      sdsEnabled: false,
      sdsVariantId: "",
      sdsParentProductId: "",
      sdsPrototypeGroupId: "",
      sdsLayerId: "",
      sdsDesignType: "material",
      sdsFitLevel: "1",
      sdsResizeMode: "0",
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

    fireEvent.click(screen.getByRole("tab", { name: "商品链接" }));
    expect(screen.getByText("选填标题")).toBeInTheDocument();
    expect(
      screen.getByText("如果已经提供商品链接，这里不是必填；只有想覆盖原始标题时再填写。"),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("商品链接"), {
      target: { value: "https://detail.1688.com/offer/123456789.html" },
    });
    fireEvent.click(screen.getByLabelText("Temu"));

    fireEvent.click(screen.getByRole("button", { name: "创建任务" }));

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith({
        text: "",
        image_urls: [],
        product_url: "https://detail.1688.com/offer/123456789.html",
        platforms: ["temu"],
      });
    });
  });

  it("shows productized input guidance", () => {
    render(<TaskCreateForm />);

    expect(screen.getByText("商品标题")).toBeInTheDocument();
    expect(
      screen.getByText("适合从图片开始创建任务。标题越完整，生成质量通常越稳定。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("先提供标题、图片或商品链接，再选择要生成的平台。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("可以直接粘贴公网图片链接、上传本地图片，或改用商品链接开始。"),
    ).toBeInTheDocument();
    expect(screen.getByText("输入建议")).toBeInTheDocument();
    expect(screen.getByText("至少提供 3 张图片")).toBeInTheDocument();
    expect(screen.getByText("标题或文案建议达到 50 个字符")).toBeInTheDocument();
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

    expect(screen.getByLabelText("图片链接")).toHaveFocus();
    expect(
      screen.getByText("上一次任务失败在图片覆盖不足，请至少补充 3 张清晰商品图。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("上一次任务失败在文案质量不足，请补充更完整的标题或描述。"),
    ).toBeInTheDocument();
  });

  it("lets the user jump directly to URL or image source fields", async () => {
    render(<TaskCreateForm />);

    fireEvent.click(screen.getByRole("tab", { name: "商品链接" }));
    await waitFor(() => {
      expect(screen.getByLabelText("商品链接")).toHaveFocus();
    });

    fireEvent.click(screen.getByRole("tab", { name: "图片素材" }));
    await waitFor(() => {
      expect(screen.getByLabelText("图片链接")).toHaveFocus();
    });
  });

  it("blocks submit when no source input is provided", async () => {
    render(<TaskCreateForm />);

    fireEvent.click(screen.getByLabelText("SHEIN"));
    fireEvent.click(screen.getByRole("button", { name: "创建任务" }));

    await waitFor(() => {
      expect(
        screen.getByText("请至少提供商品标题、图片链接或商品链接中的一种。"),
      ).toBeInTheDocument();
    });
  });

  it("uploads local files and appends returned URLs", async () => {
    uploadMutateAsync.mockResolvedValue({
      image_urls: [
        "http://localhost:8080/api/v1/listing-kits/uploads/files/a.jpg",
        "http://localhost:8080/api/v1/listing-kits/uploads/files/b.jpg",
      ],
    });

    render(<TaskCreateForm />);

    const fileInput = screen.getByLabelText("上传图片");
    const firstFile = new File(["a"], "a.jpg", { type: "image/jpeg" });
    const secondFile = new File(["b"], "b.png", { type: "image/png" });
    fireEvent.change(fileInput, {
      target: { files: [firstFile, secondFile] },
    });

    fireEvent.click(screen.getByRole("button", { name: "上传所选图片" }));

    await waitFor(() => {
      expect(uploadMutateAsync).toHaveBeenCalledWith([firstFile, secondFile]);
    });

    expect(screen.getByLabelText("图片链接")).toHaveValue(
      "http://localhost:8080/api/v1/listing-kits/uploads/files/a.jpg\nhttp://localhost:8080/api/v1/listing-kits/uploads/files/b.jpg",
    );
  });
});
