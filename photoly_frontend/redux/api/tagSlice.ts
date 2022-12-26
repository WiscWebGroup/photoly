import {apiSlice***REMOVED*** from "./apiSlice";
import { serverTag ***REMOVED*** from "../types/tagInterface";
import { ServerInterface ***REMOVED*** from "../types/serverInterface";

const extendedTagApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getAllTags: builder.query<serverTag[], void>({
      query: () => '/tag/getAll',
      transformResponse(response: ServerInterface<serverTag[]>) {
        return response.t
  ***REMOVED***,
      providesTags: ['tag']
***REMOVED***),
    insertTag: builder.mutation({
      query: (tag_name: string) => ({
        url: '/tag/insert',
        method: 'POST',
        body: {
          tagName: tag_name
***REMOVED***
  ***REMOVED***),
      invalidatesTags: ['tag']
***REMOVED***),
    deleteTag: builder.mutation({
      query: (tag_id: number) => ({
        url: `/tag/delete?tagId=${tag_id***REMOVED***`,
        method: 'POST'
  ***REMOVED***),
      invalidatesTags: ['tag']
***REMOVED***),
    renameTag: builder.mutation({
      query: ({tag_id, new_name***REMOVED***: {tag_id: number, new_name: string***REMOVED***) => ({
        url: '/tag/update',
        method: 'POST',
        body: {
          tagId: tag_id,
          tagName: new_name
***REMOVED***
  ***REMOVED***),
      invalidatesTags: ['tag']
***REMOVED***)
  ***REMOVED***)
***REMOVED***)

export const {
  useGetAllTagsQuery,
  useInsertTagMutation,
  useDeleteTagMutation,
  useRenameTagMutation
***REMOVED*** = extendedTagApi