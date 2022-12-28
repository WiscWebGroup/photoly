import {apiSlice***REMOVED*** from "./apiSlice";
import {ServerInterface***REMOVED*** from "../types/serverInterface";
import { serverPhoto ***REMOVED*** from "../types/photoInterface";

const extendedPhotoApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getPhotoByNamespace: builder.query<serverPhoto[], number>({
      query: (ns_id: number) => `/photo/getByNamespace?nsId=${ns_id***REMOVED***`,
      transformResponse(response: ServerInterface<serverPhoto[]>) {
        return response.t
  ***REMOVED***
***REMOVED***),
  ***REMOVED***)
***REMOVED***)

export const {
  useGetPhotoByNamespaceQuery
***REMOVED*** = extendedPhotoApi