"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useForm, useWatch } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import {
  saveTaskCreateDraft,
  type TaskCreateDraft,
} from "@/components/listingkit/task-create-draft";
import { TaskInputGuidance } from "@/components/listingkit/task-input-guidance";
import {
  TaskSourceTabs,
  type TaskSourceTab,
} from "@/components/listingkit/task-source-tabs";
import { useCreateTask } from "@/lib/query/use-create-task";
import { useUploadImages } from "@/lib/query/use-upload-images";

const platformOptions = [
  { value: "amazon", label: "Amazon" },
  { value: "shein", label: "Shein" },
  { value: "temu", label: "Temu" },
  { value: "walmart", label: "Walmart" },
] as const;

const schema = z
  .object({
    text: z.string().trim(),
    imageUrls: z.string().trim(),
    productUrl: z.string().trim(),
    platforms: z.array(z.string()).min(1, "Select at least one platform."),
  });

type FormValues = z.infer<typeof schema>;

function parseImageUrls(input: string) {
  return input
    .split(/\r?\n/)
    .map((value) => value.trim())
    .filter(Boolean);
}

function inferInitialSourceTab({
  initialValues,
  initialFocus,
}: {
  initialValues?: Partial<TaskCreateDraft>;
  initialFocus?: "text" | "imageUrls" | "productUrl";
}): TaskSourceTab {
  if (initialFocus === "productUrl") {
    return "productUrl";
  }
  if (initialFocus === "imageUrls") {
    return "imageUrls";
  }
  if (initialValues?.productUrl?.trim()) {
    return "productUrl";
  }

  return "imageUrls";
}

function titleFieldCopy(activeSourceTab: TaskSourceTab) {
  if (activeSourceTab === "productUrl") {
    return {
      label: "Optional title",
      helper:
        "Not required when you provide a product URL. Add it only if you want to override or improve the listing title.",
    };
  }

  return {
    label: "Product title",
    helper:
      "Recommended for image-driven creation. Stronger title text helps ListingKit pass the current quality gate.",
  };
}

export function TaskCreateForm({
  initialValues,
  initialFocus,
  fieldIssues,
}: {
  initialValues?: Partial<TaskCreateDraft>;
  initialFocus?: "text" | "imageUrls" | "productUrl";
  fieldIssues?: Array<"text" | "imageUrls" | "productUrl">;
}) {
  const router = useRouter();
  const createTask = useCreateTask();
  const uploadImages = useUploadImages();
  const textRef = useRef<HTMLInputElement | null>(null);
  const imageUrlsRef = useRef<HTMLTextAreaElement | null>(null);
  const productUrlRef = useRef<HTMLInputElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [activeSourceTab, setActiveSourceTab] = useState<TaskSourceTab>(() =>
    inferInitialSourceTab({ initialValues, initialFocus }),
  );
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const {
    register,
    handleSubmit,
    setError,
    clearErrors,
    setValue,
    formState: { errors },
    control,
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      text: initialValues?.text ?? "",
      imageUrls: initialValues?.imageUrls ?? "",
      productUrl: initialValues?.productUrl ?? "",
      platforms: initialValues?.platforms ?? [],
    },
  });

  const selectedPlatforms = useWatch({
    control,
    name: "platforms",
  });
  const currentText = useWatch({
    control,
    name: "text",
  });
  const currentImageUrls = useWatch({
    control,
    name: "imageUrls",
  });
  const currentProductUrl = useWatch({
    control,
    name: "productUrl",
  });
  const helperText = useMemo(
    () =>
      "Use public image URLs, upload local image files, or paste a product URL such as a 1688 listing.",
    [],
  );
  const imageCount = useMemo(
    () => parseImageUrls(currentImageUrls ?? "").length,
    [currentImageUrls],
  );
  const textLength = (currentText ?? "").trim().length;
  const textRegistration = register("text");
  const imageUrlsRegistration = register("imageUrls");
  const productUrlRegistration = register("productUrl");
  const titleCopy = titleFieldCopy(activeSourceTab);

  useEffect(() => {
    if (initialFocus === "text") {
      textRef.current?.focus();
    }
  }, [initialFocus]);

  useEffect(() => {
    if (activeSourceTab === "productUrl") {
      productUrlRef.current?.focus();
      return;
    }
    imageUrlsRef.current?.focus();
  }, [activeSourceTab]);

  return (
    <Card className="mx-auto max-w-3xl p-8">
      <form
        className="space-y-6"
        onSubmit={handleSubmit(async (values) => {
          const text = (values.text ?? "").trim();
          const imageUrls = values.imageUrls ?? "";
          const productUrl = (values.productUrl ?? "").trim();
          const parsedImageUrls = parseImageUrls(imageUrls);
          if (!text && parsedImageUrls.length === 0 && !productUrl) {
            setError("root", {
              message:
                "Provide at least one of product title, image URLs, or product URL.",
            });
            return;
          }
          clearErrors("root");
          const draft = {
            text,
            imageUrls,
            productUrl,
            platforms: values.platforms,
          } satisfies TaskCreateDraft;
          const request = {
            text: draft.text,
            image_urls: parsedImageUrls,
            platforms: values.platforms,
            ...(draft.productUrl ? { product_url: draft.productUrl } : {}),
          };
          const task = await createTask.mutateAsync(request);
          saveTaskCreateDraft(task.task_id, draft);
          router.push(`/listing-kits/${task.task_id}/status`);
        })}
      >
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
            ListingKit UI
          </p>
          <h1 className="text-3xl font-semibold tracking-tight text-zinc-950">
            Create ListingKit task
          </h1>
          <p className="text-sm leading-6 text-zinc-600">
            Start with a product title, image URLs, or a product URL such as a
            1688 listing, then choose the target platforms.
          </p>
        </div>

        <TaskSourceTabs
          activeTab={activeSourceTab}
          onTabChange={(tab) => {
            setActiveSourceTab(tab);
          }}
        />

        <label className="block space-y-2">
          <span className="text-sm font-medium text-zinc-700">{titleCopy.label}</span>
          <input
            aria-label="Product title"
            className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
            placeholder="Women knit cardigan"
            {...textRegistration}
            ref={(element) => {
              textRef.current = element;
              textRegistration.ref(element);
            }}
          />
          <p className="text-sm leading-6 text-zinc-500">{titleCopy.helper}</p>
          {errors.text ? (
            <p className="text-sm text-red-600">{errors.text.message}</p>
          ) : null}
          {fieldIssues?.includes("text") ? (
            <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
              The previous task failed on product copy quality. Expand the title or
              description input.
            </p>
          ) : null}
        </label>

        {activeSourceTab === "imageUrls" ? (
          <label className="block space-y-2">
            <span className="text-sm font-medium text-zinc-700">Image URLs</span>
            <div className="rounded-2xl border border-dashed border-zinc-300 bg-zinc-50 px-4 py-4">
              <div className="flex flex-wrap items-center gap-3">
                <input
                  aria-label="Upload images"
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
                  {uploadImages.isPending ? "Uploading..." : "Upload selected images"}
                </Button>
              </div>
              <p className="mt-3 text-sm leading-6 text-zinc-500">
                Choose local image files, upload them first, then ListingKit will
                append the returned URLs into the field below.
              </p>
              {selectedFiles.length > 0 ? (
                <p className="mt-2 text-sm text-zinc-700">
                  Selected {selectedFiles.length} file
                  {selectedFiles.length > 1 ? "s" : ""}:{" "}
                  {selectedFiles.map((file) => file.name).join(", ")}
                </p>
              ) : null}
            </div>
            <textarea
              aria-label="Image URLs"
              className="min-h-40 w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              placeholder={"https://example.com/1.jpg\nhttps://example.com/2.jpg"}
              {...imageUrlsRegistration}
              ref={(element) => {
                imageUrlsRef.current = element;
                imageUrlsRegistration.ref(element);
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
                The previous task failed on image coverage. Add at least 3 strong
                product images.
              </p>
            ) : null}
          </label>
        ) : null}

        {activeSourceTab === "productUrl" ? (
          <label className="block space-y-2">
            <span className="text-sm font-medium text-zinc-700">Product URL</span>
            <input
              aria-label="Product URL"
              className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              placeholder="https://detail.1688.com/offer/123456789.html"
              {...productUrlRegistration}
              ref={(element) => {
                productUrlRef.current = element;
                productUrlRegistration.ref(element);
              }}
            />
            <p className="text-sm leading-6 text-zinc-500">
              Optional. Paste a 1688 or other product page URL when you want
              ListingKit to start from the original listing.
            </p>
            {fieldIssues?.includes("productUrl") ? (
              <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
                The previous task needs a stronger product source. Add a product URL
                so the next run can start from the original listing.
              </p>
            ) : null}
          </label>
        ) : null}

        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-zinc-700">Platforms</legend>
          <div className="grid gap-3 md:grid-cols-2">
            {platformOptions.map((platform) => (
              <label
                className="flex items-center gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-900"
                key={platform.value}
              >
                <input
                  aria-label={platform.label}
                  className="h-4 w-4 rounded border-zinc-300"
                  type="checkbox"
                  value={platform.value}
                  {...register("platforms")}
                />
                <span>{platform.label}</span>
              </label>
            ))}
          </div>
          <p className="text-sm text-zinc-500">
            Selected: {selectedPlatforms?.length ?? 0}
          </p>
          {errors.platforms ? (
            <p className="text-sm text-red-600">{errors.platforms.message}</p>
          ) : null}
        </fieldset>

        {errors.root?.message ? (
          <p className="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm leading-6 text-red-700">
            {errors.root.message}
          </p>
        ) : null}

        <TaskInputGuidance
          imageCount={imageCount}
          textLength={textLength}
          hasProductUrl={Boolean(currentProductUrl?.trim())}
        />

        <div className="flex flex-wrap gap-3">
          <Button disabled={createTask.isPending} type="submit">
            {createTask.isPending ? "Creating..." : "Create task"}
          </Button>
          <Button tone="secondary" onClick={() => router.push("/")} type="button">
            Cancel
          </Button>
        </div>
      </form>
    </Card>
  );
}
