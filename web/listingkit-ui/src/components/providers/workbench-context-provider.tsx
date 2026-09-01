"use client";

import { useIsMutating, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useCallback,
  useRef,
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
  registerOrganizationSwitchGuard: (
    guard: (target: WorkbenchOrganization) => boolean | Promise<boolean>,
  ) => () => void;
};

const WorkbenchContextState = createContext<WorkbenchContextValue | null>(null);

export function WorkbenchContextProvider({ children }: PropsWithChildren) {
  const queryClient = useQueryClient();
  const [blockingError, setBlockingError] =
    useState<WorkbenchContextError | null>(null);
  const guardsRef = useRef(new Set<(target: WorkbenchOrganization) => boolean | Promise<boolean>>());
  const currentContextRef = useRef<WorkbenchContext | null>(null);
  const switchRequestPendingRef = useRef(false);
  const [switchPreparing, setSwitchPreparing] = useState(false);
  const contextQuery = useQuery({
    queryKey: WORKBENCH_CONTEXT_QUERY_KEY,
    queryFn: ({ signal }) => fetchWorkbenchContext(signal),
    enabled: !blockingError && !switchPreparing,
    staleTime: 30_000,
    refetchInterval: 30_000,
    refetchOnWindowFocus: true,
  });
  const switchMutation = useMutation({
    mutationFn: switchEffectiveOrganization,
    onSuccess: (nextContext) => {
      switchRequestPendingRef.current = false;
      setSwitchPreparing(false);
      queryClient.clear();
      queryClient.setQueryData(WORKBENCH_CONTEXT_QUERY_KEY, nextContext);
      setBlockingError(null);
    },
    onError: (error) => {
      switchRequestPendingRef.current = false;
      setSwitchPreparing(false);
      queryClient.clear();
      setBlockingError(normalizeWorkbenchError(error));
    },
  });

  const currentContext = blockingError
    ? null
    : (contextQuery.data ?? null);
  useEffect(() => {
    currentContextRef.current = currentContext;
  }, [currentContext]);
  const effectiveOrganization = currentContext?.effectiveOrganizationId
    ? (currentContext.organizations.find(
        (organization) =>
          organization.id === currentContext.effectiveOrganizationId,
      ) ?? null)
    : null;
  const queryError = contextQuery.error
    ? normalizeWorkbenchError(contextQuery.error)
    : null;
  const storeMutationsPending = useIsMutating({
    predicate: isPendingStoreMutation,
  });
  const hasPendingStoreMutation = useCallback(
    () => queryClient.isMutating({ predicate: isPendingStoreMutation }) > 0,
    [queryClient],
  );
  const registerOrganizationSwitchGuard = useCallback(
    (guard: (target: WorkbenchOrganization) => boolean | Promise<boolean>) => {
      guardsRef.current.add(guard);
      return () => {
        guardsRef.current.delete(guard);
      };
    },
    [],
  );
  const switchOrganization = useCallback(
    (organizationId: string) => {
      void (async () => {
        const context = currentContextRef.current;
        const target = context?.organizations.find(
          (organization) => organization.id === organizationId,
        );
        if (
          !target ||
          target.id === context?.effectiveOrganizationId ||
          switchRequestPendingRef.current ||
          hasPendingStoreMutation()
        ) {
          return;
        }
        for (const guard of [...guardsRef.current]) {
          try {
            if (!(await guard(target))) return;
          } catch {
            return;
          }
        }
        if (switchRequestPendingRef.current || hasPendingStoreMutation()) return;
        setSwitchPreparing(true);
        switchRequestPendingRef.current = true;
        await queryClient.cancelQueries({ queryKey: WORKBENCH_CONTEXT_QUERY_KEY });
        if (hasPendingStoreMutation()) {
          switchRequestPendingRef.current = false;
          setSwitchPreparing(false);
          return;
        }
        switchMutation.mutate(organizationId);
      })();
    },
    [hasPendingStoreMutation, queryClient, switchMutation],
  );

  const value = useMemo<WorkbenchContextValue>(
    () => ({
      user: currentContext?.user ?? null,
      homeOrganizationId: currentContext?.homeOrganizationId ?? null,
      organizations: currentContext?.organizations ?? [],
      effectiveOrganization,
      roles: effectiveOrganization?.roles ?? [],
      selectionRequired: currentContext?.selectionRequired ?? false,
      isLoading: !blockingError && contextQuery.isPending,
      isSwitching: switchMutation.isPending || storeMutationsPending > 0,
      error: queryError,
      blockingError,
      retry: () => {
        setBlockingError(null);
        void contextQuery.refetch();
      },
      switchOrganization,
      registerOrganizationSwitchGuard,
    }),
    [
      blockingError,
      contextQuery,
      currentContext,
      effectiveOrganization,
      queryError,
      switchMutation,
      storeMutationsPending,
      switchOrganization,
      registerOrganizationSwitchGuard,
    ],
  );

  return (
    <WorkbenchContextState.Provider value={value}>
      {children}
    </WorkbenchContextState.Provider>
  );
}

function isPendingStoreMutation(mutation: {
  options: { mutationKey?: readonly unknown[] };
  state: { status: string };
}) {
  const key = mutation.options.mutationKey;
  return (
    mutation.state.status === "pending" &&
    Array.isArray(key) &&
    key.length === 5 &&
    key[0] === "workbench" &&
    typeof key[1] === "string" &&
    key[2] === "stores" &&
    key[3] === "mutation" &&
    typeof key[4] === "string"
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
