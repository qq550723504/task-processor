import { firstConnectedAIEntry, type AIWorkbenchEntryCapabilities } from "./ai-workbench-entry";

export type ConsolePrimaryCapabilities = AIWorkbenchEntryCapabilities & {
  operationsCockpitAvailable?: boolean;
  cockpitPermissions?: readonly string[];
  productAcquisitionAvailable?: boolean;
  supplyMarketAvailable?: boolean;
  supplyChainAvailable?: boolean;
  dataServicesAvailable?: boolean;
  productCollectionsAvailable?: boolean;
  toolMarketAvailable?: boolean;
  ecoservicesAvailable?: boolean;
};

// Fixed current Console entries. Choosing an entry never authorizes its owner.
export function firstConnectedConsoleEntry(pathname: string, capabilities: ConsolePrimaryCapabilities): string | null {
  switch (pathname) {
    case "/workbench":
      if (!capabilities.operationsCockpitAvailable) return null;
      for (const mode of ["goals", "stores", "alerts", "advice"]) {
        if (capabilities.cockpitPermissions?.includes(`workbench.cockpit.${mode}.read`)) return `/workbench/overview/${mode}`;
      }
      return null;
    case "/workbench/ai": return firstConnectedAIEntry(capabilities);
    case "/workbench/supply":
      if (capabilities.productAcquisitionAvailable) return "/workbench/supply/acquisition";
      if (capabilities.supplyMarketAvailable) return "/workbench/supply/official";
      return capabilities.supplyChainAvailable ? "/workbench/supply/mine" : null;
    case "/workbench/agents": return "/workbench/agents/market";
    case "/workbench/tools": return capabilities.toolMarketAvailable ? "/workbench/tools/official" : null;
    case "/workbench/services": return capabilities.ecoservicesAvailable ? "/workbench/services/market" : null;
    case "/workbench/data":
      if (capabilities.dataServicesAvailable) return "/workbench/data/market";
      return capabilities.productCollectionsAvailable ? "/workbench/data/mine" : null;
    case "/workbench/store-center": return "/workbench/stores";
    case "/workbench/plans": return "/workbench/plans";
    case "/workbench/account": return "/workbench/account";
    default: return null;
  }
}
