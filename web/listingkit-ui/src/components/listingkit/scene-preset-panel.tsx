import type { ScenePresetSummary } from "@/lib/types/listingkit";

function hasScenePreset(summary?: ScenePresetSummary | null): boolean {
  if (!summary) {
    return false;
  }
  return Boolean(
    summary.prompt_key ||
      summary.defaults_source ||
      summary.scene_category ||
      summary.scene_style ||
      summary.background_tone ||
      summary.composition ||
      summary.props_level ||
      summary.audience_hint ||
      summary.custom_scene_hint,
  );
}

function ScenePresetRow({
  label,
  value,
}: {
  label: string;
  value?: string;
}) {
  if (!value) {
    return null;
  }
  return (
    <div className="grid gap-1 rounded-xl border border-zinc-200/80 bg-white/80 px-3 py-2">
      <dt className="text-[11px] font-medium uppercase tracking-[0.18em] text-zinc-500">
        {label}
      </dt>
      <dd className="text-sm text-zinc-800">{value}</dd>
    </div>
  );
}

export function ScenePresetPanel({
  summary,
}: {
  summary?: ScenePresetSummary | null;
}) {
  if (!hasScenePreset(summary)) {
    return null;
  }

  return (
    <section className="rounded-2xl border border-zinc-200 bg-zinc-50/80 p-4">
      <div className="space-y-1">
        <h2 className="text-sm font-semibold text-zinc-900">Scene preset</h2>
        <p className="text-sm text-zinc-600">
          Active scene generation settings for the focused asset.
        </p>
      </div>

      <dl className="mt-4 grid gap-3">
        <ScenePresetRow label="Prompt key" value={summary?.prompt_key} />
        <ScenePresetRow
          label="Defaults source"
          value={summary?.defaults_source}
        />
        <ScenePresetRow label="Scene category" value={summary?.scene_category} />
        <ScenePresetRow label="Scene style" value={summary?.scene_style} />
        <ScenePresetRow
          label="Background tone"
          value={summary?.background_tone}
        />
        <ScenePresetRow label="Composition" value={summary?.composition} />
        <ScenePresetRow label="Props level" value={summary?.props_level} />
        <ScenePresetRow label="Audience hint" value={summary?.audience_hint} />
        <ScenePresetRow
          label="Custom scene hint"
          value={summary?.custom_scene_hint}
        />
      </dl>
    </section>
  );
}
