import type { ReactNode, RefObject } from "react";

import { SheinCreatedTasksList } from "@/components/listingkit/shein-studio/shein-created-tasks-list";
import { SheinSavedBatchesPanel } from "@/components/listingkit/shein-studio/shein-saved-batches-panel";
import { Button } from "@/components/shared/button";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
import { SHEIN_STUDIO_PRODUCT_IMAGE_ROLES } from "@/lib/shein-studio/storage-shared";
import type {
  SheinStudioCreatedTask,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

export function SheinStudioGenerationPanel({
  createdTasks,
  creatingError,
  creatingMessage,
  generationError,
  imageStrategy,
  isCreatingTasks,
  isGenerating,
  onCreateTasks,
  onDeleteBatch,
  onGenerate,
  onLoadBatch,
  onSaveBatch,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  prompt,
  promptInputRef,
  savedBatches,
  saveMessage,
  selectedStyleCount,
  selectionReady,
  setImageStrategy,
  setProductImageCount,
  setProductImagePrompt,
  setProductImagePrompts,
  setPrompt,
  setSheinStoreId,
  setStyleCount,
  sheinStoreId,
  styleCount,
}: {
  createdTasks: SheinStudioCreatedTask[];
  creatingError: string;
  creatingMessage: string;
  generationError: string;
  imageStrategy: SheinStudioImageStrategy;
  isCreatingTasks: boolean;
  isGenerating: boolean;
  onCreateTasks: () => void;
  onDeleteBatch: (batchID: string) => void;
  onGenerate: () => void;
  onLoadBatch: (batch: SheinStudioSavedBatch) => void;
  onSaveBatch: () => void;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  prompt: string;
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  savedBatches: SheinStudioSavedBatch[];
  saveMessage: string;
  selectedStyleCount: number;
  selectionReady: boolean;
  setImageStrategy: (value: SheinStudioImageStrategy) => void;
  setProductImageCount: (value: string) => void;
  setProductImagePrompt: (value: string) => void;
  setProductImagePrompts: (value: SheinStudioProductImagePrompt[]) => void;
  setPrompt: (value: string) => void;
  setSheinStoreId: (value: string) => void;
  setStyleCount: (value: string) => void;
  sheinStoreId: string;
  styleCount: string;
}) {
  return (
    <div
      id="shein-studio-generator"
      className="scroll-mt-6 space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
            Step 2 · Generate
          </p>
          <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
            Artwork and product-image settings.
          </h2>
        </div>
        <div className="rounded-full bg-zinc-100 px-3 py-1 text-xs font-semibold text-zinc-600">
          {selectionReady ? "Product ready" : "Select product first"}
        </div>
      </div>

      <div className="grid gap-4">
        <div className="space-y-4 rounded-[1.5rem] border border-emerald-200 bg-[linear-gradient(135deg,_#ecfdf5,_#f8fafc)] px-4 py-4">
          <SectionHeading
            eyebrow="Artwork"
            title="Generate POD style artwork"
            description="This creates the flat design only. Product-image prompts are handled in the next block."
          />
          <label className="space-y-2">
            <span className="text-sm font-medium text-zinc-700">
              Theme prompt <span className="text-rose-600">*</span>
            </span>
            <textarea
              className="min-h-40 w-full rounded-2xl border border-emerald-200 bg-white/80 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-emerald-900 focus:bg-white"
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Retro varsity cherries, bold collegiate typography, spring palette."
              ref={promptInputRef}
              value={prompt}
            />
            <p className="text-xs leading-6 text-zinc-600">
              Studio biases generation toward print-safe graphics: larger shapes,
              cleaner contrast, and fewer tiny details or thin lines.
            </p>
          </label>
          <NumberInput
            label="Style count"
            max={8}
            min={1}
            setValue={setStyleCount}
            value={styleCount}
          />
        </div>

        <div className="space-y-4 rounded-[1.5rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
          <SectionHeading
            eyebrow="Product images"
            title="Configure marketplace image generation"
            description="These settings are used when approved artwork is turned into SHEIN data."
          />
          <div className="grid gap-4 lg:grid-cols-2">
            <NumberInput
              label="Product image count"
              max={9}
              min={1}
              setValue={setProductImageCount}
              value={productImageCount}
            />
            <label className="space-y-2">
              <span className="text-sm font-medium text-zinc-700">SHEIN store ID</span>
              <input
                className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                inputMode="numeric"
                onChange={(event) => setSheinStoreId(event.target.value)}
                placeholder={DEFAULT_SHEIN_STORE_ID}
                value={sheinStoreId}
              />
            </label>
          </div>

          <label className="space-y-2">
            <span className="text-sm font-medium text-zinc-700">Image strategy</span>
            <select
              className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              onChange={(event) =>
                setImageStrategy(event.target.value as SheinStudioImageStrategy)
              }
              value={imageStrategy}
            >
              <option value="ai_generated">AI generated product images</option>
              <option value="sds_official">SDS official render</option>
              <option value="hybrid">Hybrid: SDS + AI gallery</option>
            </select>
            <p className="text-xs leading-6 text-zinc-500">
              AI generated skips SDS designer. SDS official keeps template render.
              Hybrid uses SDS mockups first and appends AI gallery images.
            </p>
          </label>

          <label className="space-y-2">
            <span className="text-sm font-medium text-zinc-700">
              Global product image override
            </span>
            <textarea
              className="min-h-24 w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              onChange={(event) => setProductImagePrompt(event.target.value)}
              placeholder="Optional. Applies to every product image, e.g. keep backgrounds warm and minimal."
              value={productImagePrompt}
            />
            <p className="text-xs leading-6 text-zinc-500">
              Appended to every backend Amazon-compliant image template.
            </p>
          </label>

          <ProductImagePromptPlanner
            count={productImageCount}
            prompts={productImagePrompts}
            setPrompts={setProductImagePrompts}
          />
        </div>
      </div>

      <GenerationMessages
        creatingError={creatingError}
        creatingMessage={creatingMessage}
        generationError={generationError}
        saveMessage={saveMessage}
        selectedStyleCount={selectedStyleCount}
      />

      <div className="flex flex-wrap gap-3">
        <Button disabled={isGenerating} onClick={onGenerate}>
          {isGenerating ? "Generating..." : "Generate styles"}
        </Button>
        <Button onClick={onSaveBatch} tone="ghost">
          Save batch
        </Button>
        <Button
          disabled={isCreatingTasks || selectedStyleCount === 0 || !selectionReady}
          onClick={onCreateTasks}
          tone="secondary"
        >
          {isCreatingTasks ? "Generating SHEIN data..." : "Generate SHEIN data"}
        </Button>
      </div>

      <div id="shein-created-tasks" className="scroll-mt-6">
        <SheinCreatedTasksList tasks={createdTasks} />
      </div>
      <SheinSavedBatchesPanel
        batches={savedBatches}
        onDelete={onDeleteBatch}
        onLoad={onLoadBatch}
      />
    </div>
  );
}

function ProductImagePromptPlanner({
  count,
  prompts,
  setPrompts,
}: {
  count: string;
  prompts: SheinStudioProductImagePrompt[];
  setPrompts: (value: SheinStudioProductImagePrompt[]) => void;
}) {
  const visibleCount = clampProductImageCount(count);
  const roles = SHEIN_STUDIO_PRODUCT_IMAGE_ROLES.slice(0, visibleCount);
  const promptByRole = new Map(prompts.map((item) => [item.role, item]));

  function updatePrompt(role: string, label: string, prompt: string) {
    setPrompts([
      ...prompts.filter((item) => item.role !== role),
      { label, prompt, role },
    ]);
  }

  return (
    <div className="space-y-3 rounded-[1.5rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
      <div>
        <div className="text-sm font-semibold text-zinc-800">Per-image prompts</div>
        <p className="mt-1 text-xs leading-5 text-zinc-500">
          Optional. Leave blank to use the default role template for that image.
        </p>
      </div>
      <div className="grid gap-3">
        {roles.map((role, index) => (
          <label
            className="rounded-2xl border border-zinc-200 bg-white px-3 py-3"
            key={role.role}
          >
            <span className="flex items-center justify-between gap-3 text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              <span>
                {index + 1}. {role.label}
              </span>
              <span className="text-[10px] text-zinc-400">{role.role}</span>
            </span>
            <textarea
              className="mt-2 min-h-20 w-full rounded-xl border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
              onChange={(event) =>
                updatePrompt(role.role, role.label, event.target.value)
              }
              placeholder={role.hint}
              value={promptByRole.get(role.role)?.prompt ?? role.defaultPrompt}
            />
          </label>
        ))}
      </div>
    </div>
  );
}

function SectionHeading({
  description,
  eyebrow,
  title,
}: {
  description: string;
  eyebrow: string;
  title: string;
}) {
  return (
    <div>
      <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
        {eyebrow}
      </div>
      <h3 className="mt-1 text-lg font-semibold tracking-[-0.02em] text-zinc-950">
        {title}
      </h3>
      <p className="mt-1 text-xs leading-5 text-zinc-600">{description}</p>
    </div>
  );
}

function clampProductImageCount(value: string) {
  const parsed = Number.parseInt(value.trim(), 10);
  if (!Number.isFinite(parsed)) {
    return 1;
  }
  return Math.min(9, Math.max(1, parsed));
}

function NumberInput({
  label,
  max,
  min,
  setValue,
  value,
}: {
  label: string;
  max: number;
  min: number;
  setValue: (value: string) => void;
  value: string;
}) {
  return (
    <label className="space-y-2">
      <span className="text-sm font-medium text-zinc-700">{label}</span>
      <input
        className="w-full rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
        inputMode="numeric"
        max={max}
        min={min}
        onChange={(event) => setValue(event.target.value)}
        value={value}
      />
    </label>
  );
}

function GenerationMessages({
  creatingError,
  creatingMessage,
  generationError,
  saveMessage,
  selectedStyleCount,
}: {
  creatingError: string;
  creatingMessage: string;
  generationError: string;
  saveMessage: string;
  selectedStyleCount: number;
}) {
  return (
    <>
      {generationError ? (
        <Message tone="error">{generationError}</Message>
      ) : null}
      {creatingError ? <Message tone="error">{creatingError}</Message> : null}
      {creatingMessage ? <Message tone="info">{creatingMessage}</Message> : null}
      {selectedStyleCount > 0 ? (
        <Message tone="success">
          {selectedStyleCount} style{selectedStyleCount > 1 ? "s" : ""} selected
          for SHEIN review.
        </Message>
      ) : null}
      {saveMessage ? <Message tone="neutral">{saveMessage}</Message> : null}
    </>
  );
}

function Message({
  children,
  tone,
}: {
  children: ReactNode;
  tone: "error" | "info" | "neutral" | "success";
}) {
  const classes = {
    error: "border-rose-200 bg-rose-50 text-rose-700",
    info: "border-sky-200 bg-sky-50 text-sky-800",
    neutral: "border-zinc-200 bg-zinc-50 text-zinc-600",
    success: "border-emerald-200 bg-emerald-50 text-emerald-800",
  };

  return (
    <div className={`rounded-2xl border px-4 py-3 text-sm ${classes[tone]}`}>
      {children}
    </div>
  );
}
