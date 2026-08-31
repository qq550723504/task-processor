"use client";

import {
  useMutation,
  useQuery,
  useQueryClient,
  type MutateOptions,
  type UseMutationResult,
} from "@tanstack/react-query";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import {
  WorkbenchAPIError,
  createWorkbenchStore,
  deleteWorkbenchStore,
  disableWorkbenchStore,
  enableWorkbenchStore,
  getWorkbenchStore,
  listWorkbenchStores,
  updateWorkbenchStore,
  type WorkbenchStoreCreateInput,
  type WorkbenchStore,
  type WorkbenchStoreDeleteResult,
  type WorkbenchStoreListFilters,
  type WorkbenchStoreUpdateInput,
} from "@/lib/api/workbench-stores";

export const workbenchStoreKeys = {
  root: (organizationId: string) =>
    ["workbench", organizationId, "stores"] as const,
  list: (organizationId: string, filters: WorkbenchStoreListFilters) =>
    [...workbenchStoreKeys.root(organizationId), "list", filters] as const,
  item: (organizationId: string, storeId: string) =>
    [...workbenchStoreKeys.root(organizationId), "item", storeId] as const,
};

type CapturedOperation<T> = {
  organizationId: string;
  input: T;
};

type CapturedKeyedOperation<T> = CapturedOperation<T> & {
  operationKey: string;
};

type StoreVersionInput = { id: string; version: number };
type StoreUpdateInput = StoreVersionInput & {
  input: WorkbenchStoreUpdateInput;
};

export function useWorkbenchStores(filters: WorkbenchStoreListFilters) {
  const organizationId = useEffectiveOrganizationId();
  return useQuery({
    queryKey: workbenchStoreKeys.list(organizationId, filters),
    queryFn: () => listWorkbenchStores(filters),
    enabled: organizationId.length > 0,
    retry: retryWorkbenchRequest,
  });
}

export function useWorkbenchStore(storeId: string) {
  const organizationId = useEffectiveOrganizationId();
  return useQuery({
    queryKey: workbenchStoreKeys.item(organizationId, storeId),
    queryFn: () => getWorkbenchStore(storeId),
    enabled: organizationId.length > 0 && storeId.length > 0,
    retry: retryWorkbenchRequest,
  });
}

export function useCreateWorkbenchStore() {
  const organizationId = useEffectiveOrganizationId();
  const queryClient = useQueryClient();
  const mutation = useMutation<
    WorkbenchStore,
    WorkbenchAPIError,
    CapturedKeyedOperation<WorkbenchStoreCreateInput>
  >({
    mutationFn: (operation: CapturedKeyedOperation<WorkbenchStoreCreateInput>) => {
      requireEffectiveOrganization(operation.organizationId);
      return createWorkbenchStore(operation.input, operation.operationKey);
    },
    retry: retryWorkbenchRequest,
    onSuccess: (_store, operation) =>
      invalidateCapturedOrganization(queryClient, operation.organizationId),
  });
  return {
    ...mutation,
    variables: mutation.variables?.input,
    mutate: (
      input: WorkbenchStoreCreateInput,
      options?: MutationOptions<WorkbenchStore, WorkbenchStoreCreateInput>,
    ) =>
      mutation.mutate(
        captureKeyedOperation(organizationId, input),
        captureMutationOptions(options, input),
      ),
    mutateAsync: (
      input: WorkbenchStoreCreateInput,
      options?: MutationOptions<WorkbenchStore, WorkbenchStoreCreateInput>,
    ) =>
      mutation.mutateAsync(
        captureKeyedOperation(organizationId, input),
        captureMutationOptions(options, input),
      ),
  };
}

export function useUpdateWorkbenchStore() {
  const organizationId = useEffectiveOrganizationId();
  const queryClient = useQueryClient();
  const mutation = useMutation<
    WorkbenchStore,
    WorkbenchAPIError,
    CapturedOperation<StoreUpdateInput>
  >({
    mutationFn: (operation: CapturedOperation<StoreUpdateInput>) => {
      requireEffectiveOrganization(operation.organizationId);
      const { id, input, version } = operation.input;
      return updateWorkbenchStore(id, input, version);
    },
    retry: retryWorkbenchRequest,
    onSuccess: (_store, operation) =>
      invalidateCapturedOrganization(queryClient, operation.organizationId),
  });
  return exposeMutation(mutation, (input: StoreUpdateInput) =>
    captureOperation(organizationId, input),
  );
}

export function useEnableWorkbenchStore() {
  return useStoreStateMutation(enableWorkbenchStore);
}

export function useDisableWorkbenchStore() {
  return useStoreStateMutation(disableWorkbenchStore);
}

export function useDeleteWorkbenchStore() {
  const organizationId = useEffectiveOrganizationId();
  const queryClient = useQueryClient();
  const mutation = useMutation<
    WorkbenchStoreDeleteResult,
    WorkbenchAPIError,
    CapturedKeyedOperation<StoreVersionInput>
  >({
    mutationFn: (operation: CapturedKeyedOperation<StoreVersionInput>) => {
      requireEffectiveOrganization(operation.organizationId);
      return deleteWorkbenchStore(
        operation.input.id,
        operation.input.version,
        operation.operationKey,
      );
    },
    retry: retryWorkbenchRequest,
    onSuccess: (_result: WorkbenchStoreDeleteResult, operation) =>
      invalidateCapturedOrganization(queryClient, operation.organizationId),
  });
  return {
    ...mutation,
    variables: mutation.variables?.input,
    mutate: (
      input: StoreVersionInput,
      options?: MutationOptions<WorkbenchStoreDeleteResult, StoreVersionInput>,
    ) =>
      mutation.mutate(
        captureKeyedOperation(organizationId, input),
        captureMutationOptions(options, input),
      ),
    mutateAsync: (
      input: StoreVersionInput,
      options?: MutationOptions<WorkbenchStoreDeleteResult, StoreVersionInput>,
    ) =>
      mutation.mutateAsync(
        captureKeyedOperation(organizationId, input),
        captureMutationOptions(options, input),
      ),
  };
}

function useStoreStateMutation(
  action: (storeId: string, version: number) => ReturnType<
    typeof enableWorkbenchStore
  >,
) {
  const organizationId = useEffectiveOrganizationId();
  const queryClient = useQueryClient();
  const mutation = useMutation<
    WorkbenchStore,
    WorkbenchAPIError,
    CapturedOperation<StoreVersionInput>
  >({
    mutationFn: (operation: CapturedOperation<StoreVersionInput>) => {
      requireEffectiveOrganization(operation.organizationId);
      return action(operation.input.id, operation.input.version);
    },
    retry: retryWorkbenchRequest,
    onSuccess: (_store, operation) =>
      invalidateCapturedOrganization(queryClient, operation.organizationId),
  });
  return exposeMutation(mutation, (input: StoreVersionInput) =>
    captureOperation(organizationId, input),
  );
}

type MutationOptions<TData, TInput> = MutateOptions<
  TData,
  WorkbenchAPIError,
  TInput,
  unknown
>;

function exposeMutation<
  TData,
  TInput,
  TOperation extends CapturedOperation<TInput>,
>(
  mutation: UseMutationResult<TData, WorkbenchAPIError, TOperation, unknown>,
  capture: (input: TInput) => TOperation,
) {
  return {
    ...mutation,
    variables: mutation.variables?.input,
    mutate: (input: TInput, options?: MutationOptions<TData, TInput>) =>
      mutation.mutate(capture(input), captureMutationOptions(options, input)),
    mutateAsync: (input: TInput, options?: MutationOptions<TData, TInput>) =>
      mutation.mutateAsync(
        capture(input),
        captureMutationOptions(options, input),
      ),
  };
}

function captureKeyedOperation<T>(
  organizationId: string,
  input: T,
): CapturedKeyedOperation<T> {
  return {
    organizationId,
    input,
    operationKey: globalThis.crypto.randomUUID(),
  };
}

function captureOperation<T>(organizationId: string, input: T): CapturedOperation<T> {
  return { organizationId, input };
}

function requireEffectiveOrganization(organizationId: string) {
  if (organizationId) return;
  throw new WorkbenchAPIError(
    409,
    "ORGANIZATION_SELECTION_REQUIRED",
    "An Organization selection is required",
    "",
    [],
  );
}

function captureMutationOptions<TData, TInput, TOperation>(
  options: MutationOptions<TData, TInput> | undefined,
  input: TInput,
): MutateOptions<TData, WorkbenchAPIError, TOperation, unknown> | undefined {
  if (!options) return undefined;
  return {
    onSuccess: options.onSuccess
      ? (data, _operation, onMutateResult, context) =>
          options.onSuccess?.(data, input, onMutateResult, context)
      : undefined,
    onError: options.onError
      ? (error, _operation, onMutateResult, context) =>
          options.onError?.(error, input, onMutateResult, context)
      : undefined,
    onSettled: options.onSettled
      ? (data, error, _operation, onMutateResult, context) =>
          options.onSettled?.(data, error, input, onMutateResult, context)
      : undefined,
  };
}

function useEffectiveOrganizationId() {
  return useWorkbenchContext().effectiveOrganization?.id ?? "";
}

function retryWorkbenchRequest(failureCount: number, error: unknown) {
  return (
    failureCount < 2 &&
    error instanceof WorkbenchAPIError &&
    (error.status === 0 || error.status >= 500)
  );
}

function invalidateCapturedOrganization(
  queryClient: ReturnType<typeof useQueryClient>,
  organizationId: string,
) {
  return queryClient.invalidateQueries({
    queryKey: workbenchStoreKeys.root(organizationId),
  });
}
