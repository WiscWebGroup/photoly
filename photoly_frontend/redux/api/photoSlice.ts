import { apiSlice ***REMOVED*** from "./apiSlice";
import { ServerInterface ***REMOVED*** from "../types/serverInterface";
import { serverPhoto ***REMOVED*** from "../types/photoInterface";

const extendedPhotoApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getPhotoByNamespace: builder.query<serverPhoto[], number>({
      query: (ns_id: number) => `/photo/getByNamespace?nsId=${ns_id***REMOVED***`,
      transformResponse: (response: ServerInterface<serverPhoto[]>) => response.t
***REMOVED***),
    getPhotoByTags: builder.query<serverPhoto[], number[]>({
      query: (tags: number[]) => {
        return `/photo/getByTags?tagIds=${tags.join(',')***REMOVED***`
  ***REMOVED***,
      transformResponse: (response: ServerInterface<serverPhoto[]>) => response.t
***REMOVED***),
    getPhotoByAlbum: builder.query<serverPhoto[], number>({
      query: (album_id: number) => `/photo/getByGallery?gaId=${album_id***REMOVED***`,
      transformResponse: (response: ServerInterface<serverPhoto[]>) => response.t
***REMOVED***)
  ***REMOVED***)
***REMOVED***)

export const {
  useGetPhotoByNamespaceQuery,
  useGetPhotoByTagsQuery,
  useGetPhotoByAlbumQuery
***REMOVED*** = extendedPhotoApi