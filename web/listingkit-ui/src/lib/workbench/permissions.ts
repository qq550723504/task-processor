export const canCreateWorkbenchStore = (permissions: readonly string[]) => permissions.includes("workbench.store.create");
export const canUpdateWorkbenchStore = (permissions: readonly string[]) => permissions.includes("workbench.store.update");
export const canDeleteWorkbenchStore = (permissions: readonly string[]) => permissions.includes("workbench.store.delete");
