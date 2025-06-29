/**
 * Generated by orval v7.10.0 🍺
 * Do not edit manually.
 * BrowserGrid API
 * BrowserGrid is a distributed browser automation platform that provides scalable browser sessions and worker pool management.
 * OpenAPI spec version: 1.0
 */
import {
  useMutation,
  useQuery
} from '@tanstack/react-query';
import type {
  DataTag,
  DefinedInitialDataOptions,
  DefinedUseQueryResult,
  MutationFunction,
  QueryClient,
  QueryFunction,
  QueryKey,
  UndefinedInitialDataOptions,
  UseMutationOptions,
  UseMutationResult,
  UseQueryOptions,
  UseQueryResult
} from '@tanstack/react-query';

import type {
  ErrorResponse,
  GetApiV1WorkpoolsParams,
  MessageResponse,
  PatchApiV1WorkpoolsIdBody,
  ScalingRequest,
  ScalingResponse,
  WorkPool,
  WorkPoolListResponse
} from '.././model';

import { customInstance } from '.././mutator';




/**
 * Get a list of all work pools with optional filtering
 * @summary List work pools
 */
export const getApiV1Workpools = (
    params?: GetApiV1WorkpoolsParams,
 signal?: AbortSignal
) => {
      
      
      return customInstance<WorkPoolListResponse>(
      {url: `/api/v1/workpools`, method: 'GET',
        params, signal
    },
      );
    }
  

export const getGetApiV1WorkpoolsQueryKey = (params?: GetApiV1WorkpoolsParams,) => {
    return [`/api/v1/workpools`, ...(params ? [params]: [])] as const;
    }

    
export const getGetApiV1WorkpoolsQueryOptions = <TData = Awaited<ReturnType<typeof getApiV1Workpools>>, TError = ErrorResponse>(params?: GetApiV1WorkpoolsParams, options?: { query?:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1Workpools>>, TError, TData>>, }
) => {

const {query: queryOptions} = options ?? {};

  const queryKey =  queryOptions?.queryKey ?? getGetApiV1WorkpoolsQueryKey(params);

  

    const queryFn: QueryFunction<Awaited<ReturnType<typeof getApiV1Workpools>>> = ({ signal }) => getApiV1Workpools(params, signal);

      

      

   return  { queryKey, queryFn,   staleTime: 10000, refetchOnWindowFocus: false,  ...queryOptions} as UseQueryOptions<Awaited<ReturnType<typeof getApiV1Workpools>>, TError, TData> & { queryKey: DataTag<QueryKey, TData, TError> }
}

export type GetApiV1WorkpoolsQueryResult = NonNullable<Awaited<ReturnType<typeof getApiV1Workpools>>>
export type GetApiV1WorkpoolsQueryError = ErrorResponse


export function useGetApiV1Workpools<TData = Awaited<ReturnType<typeof getApiV1Workpools>>, TError = ErrorResponse>(
 params: undefined |  GetApiV1WorkpoolsParams, options: { query:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1Workpools>>, TError, TData>> & Pick<
        DefinedInitialDataOptions<
          Awaited<ReturnType<typeof getApiV1Workpools>>,
          TError,
          Awaited<ReturnType<typeof getApiV1Workpools>>
        > , 'initialData'
      >, }
 , queryClient?: QueryClient
  ):  DefinedUseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> }
export function useGetApiV1Workpools<TData = Awaited<ReturnType<typeof getApiV1Workpools>>, TError = ErrorResponse>(
 params?: GetApiV1WorkpoolsParams, options?: { query?:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1Workpools>>, TError, TData>> & Pick<
        UndefinedInitialDataOptions<
          Awaited<ReturnType<typeof getApiV1Workpools>>,
          TError,
          Awaited<ReturnType<typeof getApiV1Workpools>>
        > , 'initialData'
      >, }
 , queryClient?: QueryClient
  ):  UseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> }
export function useGetApiV1Workpools<TData = Awaited<ReturnType<typeof getApiV1Workpools>>, TError = ErrorResponse>(
 params?: GetApiV1WorkpoolsParams, options?: { query?:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1Workpools>>, TError, TData>>, }
 , queryClient?: QueryClient
  ):  UseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> }
/**
 * @summary List work pools
 */

export function useGetApiV1Workpools<TData = Awaited<ReturnType<typeof getApiV1Workpools>>, TError = ErrorResponse>(
 params?: GetApiV1WorkpoolsParams, options?: { query?:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1Workpools>>, TError, TData>>, }
 , queryClient?: QueryClient 
 ):  UseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> } {

  const queryOptions = getGetApiV1WorkpoolsQueryOptions(params,options)

  const query = useQuery(queryOptions , queryClient) as  UseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> };

  query.queryKey = queryOptions.queryKey ;

  return query;
}



/**
 * Create a new work pool to manage browser workers
 * @summary Create a new work pool
 */
export const postApiV1Workpools = (
    workPool: WorkPool,
 signal?: AbortSignal
) => {
      
      
      return customInstance<WorkPool>(
      {url: `/api/v1/workpools`, method: 'POST',
      headers: {'Content-Type': 'application/json', },
      data: workPool, signal
    },
      );
    }
  


export const getPostApiV1WorkpoolsMutationOptions = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof postApiV1Workpools>>, TError,{data: WorkPool}, TContext>, }
): UseMutationOptions<Awaited<ReturnType<typeof postApiV1Workpools>>, TError,{data: WorkPool}, TContext> => {

const mutationKey = ['postApiV1Workpools'];
const {mutation: mutationOptions} = options ?
      options.mutation && 'mutationKey' in options.mutation && options.mutation.mutationKey ?
      options
      : {...options, mutation: {...options.mutation, mutationKey}}
      : {mutation: { mutationKey, }};

      


      const mutationFn: MutationFunction<Awaited<ReturnType<typeof postApiV1Workpools>>, {data: WorkPool}> = (props) => {
          const {data} = props ?? {};

          return  postApiV1Workpools(data,)
        }

        


  return  { mutationFn, ...mutationOptions }}

    export type PostApiV1WorkpoolsMutationResult = NonNullable<Awaited<ReturnType<typeof postApiV1Workpools>>>
    export type PostApiV1WorkpoolsMutationBody = WorkPool
    export type PostApiV1WorkpoolsMutationError = ErrorResponse

    /**
 * @summary Create a new work pool
 */
export const usePostApiV1Workpools = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof postApiV1Workpools>>, TError,{data: WorkPool}, TContext>, }
 , queryClient?: QueryClient): UseMutationResult<
        Awaited<ReturnType<typeof postApiV1Workpools>>,
        TError,
        {data: WorkPool},
        TContext
      > => {

      const mutationOptions = getPostApiV1WorkpoolsMutationOptions(options);

      return useMutation(mutationOptions , queryClient);
    }
    /**
 * Get details of a specific work pool by ID
 * @summary Get a work pool
 */
export const getApiV1WorkpoolsId = (
    id: string,
 signal?: AbortSignal
) => {
      
      
      return customInstance<WorkPool>(
      {url: `/api/v1/workpools/${id}`, method: 'GET', signal
    },
      );
    }
  

export const getGetApiV1WorkpoolsIdQueryKey = (id: string,) => {
    return [`/api/v1/workpools/${id}`] as const;
    }

    
export const getGetApiV1WorkpoolsIdQueryOptions = <TData = Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError = ErrorResponse>(id: string, options?: { query?:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError, TData>>, }
) => {

const {query: queryOptions} = options ?? {};

  const queryKey =  queryOptions?.queryKey ?? getGetApiV1WorkpoolsIdQueryKey(id);

  

    const queryFn: QueryFunction<Awaited<ReturnType<typeof getApiV1WorkpoolsId>>> = ({ signal }) => getApiV1WorkpoolsId(id, signal);

      

      

   return  { queryKey, queryFn, enabled: !!(id),  staleTime: 10000, refetchOnWindowFocus: false,  ...queryOptions} as UseQueryOptions<Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError, TData> & { queryKey: DataTag<QueryKey, TData, TError> }
}

export type GetApiV1WorkpoolsIdQueryResult = NonNullable<Awaited<ReturnType<typeof getApiV1WorkpoolsId>>>
export type GetApiV1WorkpoolsIdQueryError = ErrorResponse


export function useGetApiV1WorkpoolsId<TData = Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError = ErrorResponse>(
 id: string, options: { query:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError, TData>> & Pick<
        DefinedInitialDataOptions<
          Awaited<ReturnType<typeof getApiV1WorkpoolsId>>,
          TError,
          Awaited<ReturnType<typeof getApiV1WorkpoolsId>>
        > , 'initialData'
      >, }
 , queryClient?: QueryClient
  ):  DefinedUseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> }
export function useGetApiV1WorkpoolsId<TData = Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError = ErrorResponse>(
 id: string, options?: { query?:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError, TData>> & Pick<
        UndefinedInitialDataOptions<
          Awaited<ReturnType<typeof getApiV1WorkpoolsId>>,
          TError,
          Awaited<ReturnType<typeof getApiV1WorkpoolsId>>
        > , 'initialData'
      >, }
 , queryClient?: QueryClient
  ):  UseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> }
export function useGetApiV1WorkpoolsId<TData = Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError = ErrorResponse>(
 id: string, options?: { query?:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError, TData>>, }
 , queryClient?: QueryClient
  ):  UseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> }
/**
 * @summary Get a work pool
 */

export function useGetApiV1WorkpoolsId<TData = Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError = ErrorResponse>(
 id: string, options?: { query?:Partial<UseQueryOptions<Awaited<ReturnType<typeof getApiV1WorkpoolsId>>, TError, TData>>, }
 , queryClient?: QueryClient 
 ):  UseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> } {

  const queryOptions = getGetApiV1WorkpoolsIdQueryOptions(id,options)

  const query = useQuery(queryOptions , queryClient) as  UseQueryResult<TData, TError> & { queryKey: DataTag<QueryKey, TData, TError> };

  query.queryKey = queryOptions.queryKey ;

  return query;
}



/**
 * Delete an existing work pool and all its workers
 * @summary Delete a work pool
 */
export const deleteApiV1WorkpoolsId = (
    id: string,
 ) => {
      
      
      return customInstance<MessageResponse>(
      {url: `/api/v1/workpools/${id}`, method: 'DELETE'
    },
      );
    }
  


export const getDeleteApiV1WorkpoolsIdMutationOptions = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof deleteApiV1WorkpoolsId>>, TError,{id: string}, TContext>, }
): UseMutationOptions<Awaited<ReturnType<typeof deleteApiV1WorkpoolsId>>, TError,{id: string}, TContext> => {

const mutationKey = ['deleteApiV1WorkpoolsId'];
const {mutation: mutationOptions} = options ?
      options.mutation && 'mutationKey' in options.mutation && options.mutation.mutationKey ?
      options
      : {...options, mutation: {...options.mutation, mutationKey}}
      : {mutation: { mutationKey, }};

      


      const mutationFn: MutationFunction<Awaited<ReturnType<typeof deleteApiV1WorkpoolsId>>, {id: string}> = (props) => {
          const {id} = props ?? {};

          return  deleteApiV1WorkpoolsId(id,)
        }

        


  return  { mutationFn, ...mutationOptions }}

    export type DeleteApiV1WorkpoolsIdMutationResult = NonNullable<Awaited<ReturnType<typeof deleteApiV1WorkpoolsId>>>
    
    export type DeleteApiV1WorkpoolsIdMutationError = ErrorResponse

    /**
 * @summary Delete a work pool
 */
export const useDeleteApiV1WorkpoolsId = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof deleteApiV1WorkpoolsId>>, TError,{id: string}, TContext>, }
 , queryClient?: QueryClient): UseMutationResult<
        Awaited<ReturnType<typeof deleteApiV1WorkpoolsId>>,
        TError,
        {id: string},
        TContext
      > => {

      const mutationOptions = getDeleteApiV1WorkpoolsIdMutationOptions(options);

      return useMutation(mutationOptions , queryClient);
    }
    /**
 * Update configuration of an existing work pool
 * @summary Update a work pool
 */
export const patchApiV1WorkpoolsId = (
    id: string,
    patchApiV1WorkpoolsIdBody: PatchApiV1WorkpoolsIdBody,
 ) => {
      
      
      return customInstance<MessageResponse>(
      {url: `/api/v1/workpools/${id}`, method: 'PATCH',
      headers: {'Content-Type': 'application/json', },
      data: patchApiV1WorkpoolsIdBody
    },
      );
    }
  


export const getPatchApiV1WorkpoolsIdMutationOptions = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof patchApiV1WorkpoolsId>>, TError,{id: string;data: PatchApiV1WorkpoolsIdBody}, TContext>, }
): UseMutationOptions<Awaited<ReturnType<typeof patchApiV1WorkpoolsId>>, TError,{id: string;data: PatchApiV1WorkpoolsIdBody}, TContext> => {

const mutationKey = ['patchApiV1WorkpoolsId'];
const {mutation: mutationOptions} = options ?
      options.mutation && 'mutationKey' in options.mutation && options.mutation.mutationKey ?
      options
      : {...options, mutation: {...options.mutation, mutationKey}}
      : {mutation: { mutationKey, }};

      


      const mutationFn: MutationFunction<Awaited<ReturnType<typeof patchApiV1WorkpoolsId>>, {id: string;data: PatchApiV1WorkpoolsIdBody}> = (props) => {
          const {id,data} = props ?? {};

          return  patchApiV1WorkpoolsId(id,data,)
        }

        


  return  { mutationFn, ...mutationOptions }}

    export type PatchApiV1WorkpoolsIdMutationResult = NonNullable<Awaited<ReturnType<typeof patchApiV1WorkpoolsId>>>
    export type PatchApiV1WorkpoolsIdMutationBody = PatchApiV1WorkpoolsIdBody
    export type PatchApiV1WorkpoolsIdMutationError = ErrorResponse

    /**
 * @summary Update a work pool
 */
export const usePatchApiV1WorkpoolsId = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof patchApiV1WorkpoolsId>>, TError,{id: string;data: PatchApiV1WorkpoolsIdBody}, TContext>, }
 , queryClient?: QueryClient): UseMutationResult<
        Awaited<ReturnType<typeof patchApiV1WorkpoolsId>>,
        TError,
        {id: string;data: PatchApiV1WorkpoolsIdBody},
        TContext
      > => {

      const mutationOptions = getPatchApiV1WorkpoolsIdMutationOptions(options);

      return useMutation(mutationOptions , queryClient);
    }
    /**
 * Gracefully drain all workers from a work pool
 * @summary Drain a work pool
 */
export const postApiV1WorkpoolsIdDrain = (
    id: string,
 signal?: AbortSignal
) => {
      
      
      return customInstance<MessageResponse>(
      {url: `/api/v1/workpools/${id}/drain`, method: 'POST', signal
    },
      );
    }
  


export const getPostApiV1WorkpoolsIdDrainMutationOptions = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof postApiV1WorkpoolsIdDrain>>, TError,{id: string}, TContext>, }
): UseMutationOptions<Awaited<ReturnType<typeof postApiV1WorkpoolsIdDrain>>, TError,{id: string}, TContext> => {

const mutationKey = ['postApiV1WorkpoolsIdDrain'];
const {mutation: mutationOptions} = options ?
      options.mutation && 'mutationKey' in options.mutation && options.mutation.mutationKey ?
      options
      : {...options, mutation: {...options.mutation, mutationKey}}
      : {mutation: { mutationKey, }};

      


      const mutationFn: MutationFunction<Awaited<ReturnType<typeof postApiV1WorkpoolsIdDrain>>, {id: string}> = (props) => {
          const {id} = props ?? {};

          return  postApiV1WorkpoolsIdDrain(id,)
        }

        


  return  { mutationFn, ...mutationOptions }}

    export type PostApiV1WorkpoolsIdDrainMutationResult = NonNullable<Awaited<ReturnType<typeof postApiV1WorkpoolsIdDrain>>>
    
    export type PostApiV1WorkpoolsIdDrainMutationError = ErrorResponse

    /**
 * @summary Drain a work pool
 */
export const usePostApiV1WorkpoolsIdDrain = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof postApiV1WorkpoolsIdDrain>>, TError,{id: string}, TContext>, }
 , queryClient?: QueryClient): UseMutationResult<
        Awaited<ReturnType<typeof postApiV1WorkpoolsIdDrain>>,
        TError,
        {id: string},
        TContext
      > => {

      const mutationOptions = getPostApiV1WorkpoolsIdDrainMutationOptions(options);

      return useMutation(mutationOptions , queryClient);
    }
    /**
 * Update scaling parameters for a work pool
 * @summary Scale a work pool
 */
export const postApiV1WorkpoolsIdScale = (
    id: string,
    scalingRequest: ScalingRequest,
 signal?: AbortSignal
) => {
      
      
      return customInstance<ScalingResponse>(
      {url: `/api/v1/workpools/${id}/scale`, method: 'POST',
      headers: {'Content-Type': 'application/json', },
      data: scalingRequest, signal
    },
      );
    }
  


export const getPostApiV1WorkpoolsIdScaleMutationOptions = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof postApiV1WorkpoolsIdScale>>, TError,{id: string;data: ScalingRequest}, TContext>, }
): UseMutationOptions<Awaited<ReturnType<typeof postApiV1WorkpoolsIdScale>>, TError,{id: string;data: ScalingRequest}, TContext> => {

const mutationKey = ['postApiV1WorkpoolsIdScale'];
const {mutation: mutationOptions} = options ?
      options.mutation && 'mutationKey' in options.mutation && options.mutation.mutationKey ?
      options
      : {...options, mutation: {...options.mutation, mutationKey}}
      : {mutation: { mutationKey, }};

      


      const mutationFn: MutationFunction<Awaited<ReturnType<typeof postApiV1WorkpoolsIdScale>>, {id: string;data: ScalingRequest}> = (props) => {
          const {id,data} = props ?? {};

          return  postApiV1WorkpoolsIdScale(id,data,)
        }

        


  return  { mutationFn, ...mutationOptions }}

    export type PostApiV1WorkpoolsIdScaleMutationResult = NonNullable<Awaited<ReturnType<typeof postApiV1WorkpoolsIdScale>>>
    export type PostApiV1WorkpoolsIdScaleMutationBody = ScalingRequest
    export type PostApiV1WorkpoolsIdScaleMutationError = ErrorResponse

    /**
 * @summary Scale a work pool
 */
export const usePostApiV1WorkpoolsIdScale = <TError = ErrorResponse,
    TContext = unknown>(options?: { mutation?:UseMutationOptions<Awaited<ReturnType<typeof postApiV1WorkpoolsIdScale>>, TError,{id: string;data: ScalingRequest}, TContext>, }
 , queryClient?: QueryClient): UseMutationResult<
        Awaited<ReturnType<typeof postApiV1WorkpoolsIdScale>>,
        TError,
        {id: string;data: ScalingRequest},
        TContext
      > => {

      const mutationOptions = getPostApiV1WorkpoolsIdScaleMutationOptions(options);

      return useMutation(mutationOptions , queryClient);
    }
    