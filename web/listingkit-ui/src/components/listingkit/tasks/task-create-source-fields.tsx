import type { MutableRefObject } from "react";
import type {
  FieldErrors,
  UseFormRegisterReturn,
  UseFormSetValue,
} from "react-hook-form";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
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
    <Label className="block space-y-2">
      <span className="text-sm font-medium text-muted-foreground">{titleCopy.label}</span>
      <Input
        aria-label="商品标题"
        className="rounded-2xl px-4 py-3"
        placeholder="例如：女士针织开衫"
        {...registration}
        ref={(element) => {
          inputRef.current = element;
          registration.ref(element);
        }}
      />
      <p className="text-sm leading-6 text-muted-foreground">{titleCopy.helper}</p>
      {errors.text ? (
        <p className="text-sm text-red-600">{errors.text.message}</p>
      ) : null}
      {fieldIssues?.includes("text") ? (
        <Alert variant="warning">
          <AlertDescription>
            上一次任务失败在文案质量不足，请补充更完整的标题或描述。
          </AlertDescription>
        </Alert>
      ) : null}
    </Label>
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
    <Label className="block space-y-2">
      <span className="text-sm font-medium text-muted-foreground">图片链接</span>
      <div className="rounded-2xl border border-dashed border-border bg-muted px-4 py-4">
        <div className="flex flex-wrap items-center gap-3">
          <Input
            aria-label="上传图片"
            className="block text-sm text-muted-foreground file:mr-4 file:rounded-xl file:border-0 file:bg-primary file:px-4 file:py-2 file:text-sm file:font-medium file:text-primary-foreground"
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
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          可以先上传本地图片，系统会把返回的图片链接自动补到下方输入框里。
        </p>
        {selectedFiles.length > 0 ? (
          <p className="mt-2 text-sm text-foreground">
            已选择 {selectedFiles.length} 个文件：
            {selectedFiles.map((file) => file.name).join(", ")}
          </p>
        ) : null}
      </div>
      <Textarea
        aria-label="图片链接"
        className={`rounded-2xl px-4 py-3 ${
          variant === "sds" ? "min-h-28" : "min-h-40"
        }`}
        placeholder={"https://example.com/1.jpg\nhttps://example.com/2.jpg"}
        {...registration}
        ref={(element) => {
          imageUrlsRef.current = element;
          registration.ref(element);
        }}
      />
      <p className="text-sm leading-6 text-muted-foreground">{helperText}</p>
      {errors.imageUrls ? (
        <p className="text-sm text-red-600">{errors.imageUrls.message}</p>
      ) : null}
      {uploadImages.error instanceof Error ? (
        <p className="text-sm text-red-600">{uploadImages.error.message}</p>
      ) : null}
      {fieldIssues?.includes("imageUrls") ? (
        <Alert variant="warning">
          <AlertDescription>
            上一次任务失败在图片覆盖不足，请至少补充 3 张清晰商品图。
          </AlertDescription>
        </Alert>
      ) : null}
    </Label>
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
    <Label className="block space-y-2">
      <span className="text-sm font-medium text-muted-foreground">商品链接</span>
      <Input
        aria-label="商品链接"
        className="rounded-2xl px-4 py-3"
        placeholder="https://detail.1688.com/offer/123456789.html"
        {...registration}
        ref={(element) => {
          inputRef.current = element;
          registration.ref(element);
        }}
      />
      <p className="text-sm leading-6 text-muted-foreground">
        适合已有原始商品页的场景。支持 1688 等商品链接，系统会从原始商品资料开始处理。
      </p>
      {fieldIssues?.includes("productUrl") ? (
        <Alert variant="warning">
          <AlertDescription>
            上一次任务缺少足够强的商品来源，建议补充商品链接后再重试。
          </AlertDescription>
        </Alert>
      ) : null}
    </Label>
  );
}
