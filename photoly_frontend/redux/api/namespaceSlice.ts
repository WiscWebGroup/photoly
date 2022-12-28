import {apiSlice***REMOVED*** from "./apiSlice";
import { serverNamespace ***REMOVED*** from "../types/namespaceInterface";
import { ServerInterface ***REMOVED*** from "../types/serverInterface";

const extendedNamespaceApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getRoot: builder.query<serverNamespace, void>({
      query: () => '/namespace/getRoot',
      transformResponse(response: ServerInterface<serverNamespace>) {
        return response.t
  ***REMOVED***
***REMOVED***),
    getChildren: builder.query<serverNamespace[], number>({
      query: (parent_id: number) => `/namespace/getChildren?parentId=${parent_id***REMOVED***`,
      transformResponse(response: ServerInterface<serverNamespace[]>) {
        return response.t
  ***REMOVED***
***REMOVED***)
  ***REMOVED***)
***REMOVED***)

export const {
  useGetRootQuery,
  useGetChildrenQuery
***REMOVED*** = extendedNamespaceApi