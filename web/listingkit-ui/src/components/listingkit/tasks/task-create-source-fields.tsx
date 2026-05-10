import type { MutableRefObject } from "react";
import type {
  FieldErrors,
  UseFormRegisterReturn,
  UseFormSetValue,
} from "react-hook-form";

import { Button } from "@/components/shared/button";
import {
  parseImageUrls,
  type FormValues,
} from "@/components/listingkit/tasks/task-create-form-model";

export function TaskTitleField({
  errors,
  fieldIssues,
  inputRef,
  registration,
  titleCopy,
}: {
  errors: FieldErrors<FormValues>;
  fieldIssues?: Array<"text" | "imageUrls" | "productUrl">;
  inputRef: MutableRefObject<HTMLInputElement | null>;
  registration: UseFormRegisterReturn<"text">;
  titleCopy: { label: string; helper: string };
}) {
  return (
    <label className="block space-y-2">
      <span className="text-sm font-medium text-zinc-700">{titleCopy.label}</span>
      <input
        aria-label="商品标题"
        className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
        placeholder="例如：女士针织开衫"
        {...registration}
        ref={(element) => {
          inputRef.current = element;
          registration.ref(element);
        }}
      />
      <p className="text-sm leading-6 text-zinc-500">{titleCopy.helper}</p>
      {errors.text ? (
        <p className="text-sm text-red-600">{errors.text.message}</p>
      ) : null}
      {fieldIssues?.includes("text") ? (
        <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
          上一次任务失败在文案质量不足，请补充更完整的标题或描述。
        </p>
      ) : null}
    </label>
  );
}

export function TaskImageUrlField({
  currentImageUrls,
  errors,
  fieldIssues,
  fileInputRef,
  helperText,
  imageUrlsRef,
  registration,
  selectedFiles,
  setSelectedFiles,
  setValue,
  uploadImages,
  variant,
}: {
  currentImageUrls?: string;
  errors: FieldErrors<FormValues>;
  fieldIssues?: Array<"text" | "imageUrls" | "productUrl">;
  fileInputRef: MutableRefObject<HTMLInputElement | null>;
  helperText: string;
  imageUrlsRef: MutableRefObject<HTMLTextAreaElement | null>;
  registration: UseFormRegisterReturn<"imageUrls">;
  selectedFiles: File[];
  setSelectedFiles: (files: File[]) => void;
  setValue: UseFormSetValue<FormValues>;
  uploadImages: {
    error: unknown;
    isPending: boolean;
    mutateAsync: (files: File[]) => Promise<{ image_urls?: string[] }>;
  };
  variant: "default" | "sds";
}) {
  return (
    <label className="block space-y-2">
      <span className="text-sm font-medium text-zinc-700">图片链接</span>
      <div className="rounded-2xl border border-dashed border-zinc-300 bg-zinc-50 px-4 py-4">
        <div className="flex flex-wrap items-center gap-3">
          <input
            aria-label="上传图片"
            className="block text-sm text-zinc-600 file:mr-4 file:rounded-xl file:border-0 file:bg-zinc-950 file:px-4 file:py-2 file:text-sm file:font-medium file:text-white"
            multiple
            onChange={(event) => {
              const files = Array.from(event.target.files ?? []);
              setSelectedFiles(files);
            }}
            ref={fileInputRef}
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
          />
          <Button
            disabled={selectedFiles.length === 0 || uploadImages.isPending}
            onClick={async () => {
              const response = await uploadImages.mutateAsync(selectedFiles);
              const mergedUrls = [
                ...parseImageUrls(currentImageUrls ?? ""),
                ...(response.image_urls ?? []),
              ];
              setValue("imageUrls", mergedUrls.join("\n"), {
                shouldDirty: true,
              });
              setSelectedFiles([]);
              if (fileInputRef.current) {
                fileInputRef.current.value = "";
              }
            }}
            type="button"
          >
            {uploadImages.isPending ? "上传中..." : "上传所选图片"}
          </Button>
        </div>
        <p className="mt-3 text-sm leading-6 text-zinc-500">
          可以先上传本地图片，系统会把返回的图片链接自动补到下方输入框里。
        </p>
        {selectedFiles.length > 0 ? (
          <p className="mt-2 text-sm text-zinc-700">
            已选择 {selectedFiles.length} 个文件：
            {selectedFiles.map((file) => file.name).join(", ")}
          </p>
        ) : null}
      </div>
      <textarea
        aria-label="图片链接"
        className={`w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 ${
          variant === "sds" ? "min-h-28" : "min-h-40"
        }`}
        placeholder={"https://example.com/1.jpg\nhttps://example.com/2.jpg"}
        {...registration}
        ref={(element) => {
          imageUrlsRef.current = element;
          registration.ref(element);
        }}
      />
      <p className="text-sm leading-6 text-zinc-500">{helperText}</p>
      {errors.imageUrls ? (
        <p className="text-sm text-red-600">{errors.imageUrls.message}</p>
      ) : null}
      {uploadImages.error instanceof Error ? (
        <p className="text-sm text-red-600">{uploadImages.error.message}</p>
      ) : null}
      {fieldIssues?.includes("imageUrls") ? (
        <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
          上一次任务失败在图片覆盖不足，请至少补充 3 张清晰商品图。
        </p>
      ) : null}
    </label>
  );
}

export function TaskProductUrlField({
  fieldIssues,
  inputRef,
  registration,
}: {
  fieldIssues?: Array<"text" | "imageUrls" | "productUrl">;
  inputRef: MutableRefObject<HTMLInputElement | null>;
  registration: UseFormRegisterReturn<"productUrl">;
}) {
  return (
    <label className="block space-y-2">
      <span className="text-sm font-medium text-zinc-700">商品链接</span>
      <input
        aria-label="商品链接"
        className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
        placeholder="https://detail.1688.com/offer/123456789.html"
        {...registration}
        ref={(element) => {
          inputRef.current = element;
          registration.ref(element);
        }}
      />
      <p className="text-sm leading-6 text-zinc-500">
        适合已有原始商品页的场景。支持 1688 等商品链接，系统会从原始商品资料开始处理。
      </p>
      {fieldIssues?.includes("productUrl") ? (
        <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
          上一次任务缺少足够强的商品来源，建议补充商品链接后再重试。
        </p>
      ) : null}
    </label>
  );
}
