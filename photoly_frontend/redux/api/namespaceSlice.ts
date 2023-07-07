import {apiSlice} from "./apiSlice";
import { serverNamespace } from "../types/namespaceInterface";
import { ServerInterface } from "../types/serverInterface";

const extendedNamespaceApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getRoot: builder.query<serverNamespace, void>({
      query: () => '/namespace/getRoot',
      transformResponse(response: ServerInterface<serverNamespace>) {
        return response.t
      }
    }),
    getChildren: builder.query<serverNamespace[], number>({
      query: (parent_id: number) => `/namespace/getChildren?parentId=${parent_id}`,
      transformResponse(response: ServerInterface<serverNamespace[]>) {
        return response.t
      }
    })
  })
})

export const {
  useGetRootQuery,
  useLazyGetRootQuery,
  useGetChildrenQuery
} = extendedNamespaceApi