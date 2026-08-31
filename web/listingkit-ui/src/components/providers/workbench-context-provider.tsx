"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createContext,
  useContext,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

import {
  fetchWorkbenchContext,
  switchEffectiveOrganization,
  WORKBENCH_CONTEXT_QUERY_KEY,
  WorkbenchContextError,
  type WorkbenchContext,
  type WorkbenchOrganization,
} from "@/lib/api/workbench-context";

type WorkbenchContextValue = {
  user: WorkbenchContext["user"] | null;
  homeOrganizationId: string | null;
  organizations: WorkbenchOrganization[];
  effectiveOrganization: WorkbenchOrganization | null;
  roles: string[];
  selectionRequired: boolean;
  isLoading: boolean;
  isSwitching: boolean;
  error: WorkbenchContextError | null;
  blockingError: WorkbenchContextError | null;
  retry: () => void;
  switchOrganization: (organizationId: string) => void;
};

const WorkbenchContextState = createContext<WorkbenchContextValue | null>(null);

export function WorkbenchContextProvider({ children }: PropsWithChildren) {
  const queryClient = useQueryClient();
  const [switchedContext, setSwitchedContext] =
    useState<WorkbenchContext | null>(null);
  const [blockingError, setBlockingError] =
    useState<WorkbenchContextError | null>(null);
  const [explicitSwitchSettled, setExplicitSwitchSettled] = useState(false);
  const contextQuery = useQuery({
    queryKey: WORKBENCH_CONTEXT_QUERY_KEY,
    queryFn: fetchWorkbenchContext,
    enabled: !explicitSwitchSettled,
  });
  const switchMutation = useMutation({
    mutationFn: switchEffectiveOrganization,
    onSuccess: (nextContext) => {
      setExplicitSwitchSettled(true);
      queryClient.clear();
      queryClient.setQueryData(WORKBENCH_CONTEXT_QUERY_KEY, nextContext);
      setSwitchedContext(nextContext);
      setBlockingError(null);
    },
    onError: (error) => {
      setExplicitSwitchSettled(true);
      queryClient.clear();
      setSwitchedContext(null);
      setBlockingError(normalizeWorkbenchError(error));
    },
  });

  const currentContext = blockingError
    ? null
    : (switchedContext ?? contextQuery.data ?? null);
  const effectiveOrganization = currentContext?.effectiveOrganizationId
    ? (currentContext.organizations.find(
        (organization) =>
          organization.id === currentContext.effectiveOrganizationId,
      ) ?? null)
    : null;
  const queryError = contextQuery.error
    ? normalizeWorkbenchError(contextQuery.error)
    : null;

  const value = useMemo<WorkbenchContextValue>(
    () => ({
      user: currentContext?.user ?? null,
      homeOrganizationId: currentContext?.homeOrganizationId ?? null,
      organizations: currentContext?.organizations ?? [],
      effectiveOrganization,
      roles: effectiveOrganization?.roles ?? [],
      selectionRequired: currentContext?.selectionRequired ?? false,
      isLoading: !blockingError && !switchedContext && contextQuery.isPending,
      isSwitching: switchMutation.isPending,
      error: queryError,
      blockingError,
      retry: () => {
        void contextQuery.refetch();
      },
      switchOrganization: (organizationId) => {
        switchMutation.mutate(organizationId);
      },
    }),
    [
      blockingError,
      contextQuery,
      currentContext,
      effectiveOrganization,
      queryError,
      switchedContext,
      switchMutation,
    ],
  );

  return (
    <WorkbenchContextState.Provider value={value}>
      {children}
    </WorkbenchContextState.Provider>
  );
}

export function useWorkbenchContext() {
  const context = useContext(WorkbenchContextState);
  if (!context) {
    throw new Error(
      "useWorkbenchContext must be used within WorkbenchContextProvider",
    );
  }
  return context;
}

function normalizeWorkbenchError(error: unknown) {
  if (error instanceof WorkbenchContextError) return error;
  return new WorkbenchContextError(0, "WORKBENCH_REQUEST_FAILED", "", []);
}
