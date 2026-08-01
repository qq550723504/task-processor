export type WorkspacePlatformAdapter<TProjection> = {
  platform: string;
  project: () => TProjection;
};

export type WorkspacePlatformProjection<TSheinProjection = never> =
  | {
      kind: "generic";
      platform?: string;
    }
  | {
      kind: "amazon";
      platform: "amazon";
      title?: string;
      subtitle?: string;
    }
  | {
      kind: "shein";
      platform: "shein";
      projection: TSheinProjection;
    };

export function createGenericWorkspaceProjection<TSheinProjection = never>(
  platform?: string,
): WorkspacePlatformProjection<TSheinProjection> {
  return {
    kind: "generic",
    platform,
  };
}

export function resolveWorkspacePlatformAdapter<TProjection>(
  platform: string | undefined,
  adapters: readonly WorkspacePlatformAdapter<TProjection>[],
  fallback: () => TProjection,
): TProjection {
  return adapters.find((adapter) => adapter.platform === platform)?.project() ?? fallback();
}
