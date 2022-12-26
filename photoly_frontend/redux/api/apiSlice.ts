import {createApi***REMOVED*** from "@reduxjs/toolkit/query/react";
import { ITag, serverTag ***REMOVED*** from "../types/tagInterface";
import { ServerInterface ***REMOVED*** from "../types/serverInterface";
import { myBaseQuery ***REMOVED*** from "../customBaseQuery";



export const apiSlice = createApi({
  reducerPath: 'api',
  baseQuery: myBaseQuery,
  tagTypes: ['tag'],
  endpoints: builder => ({
    getAllTags: builder.query<ITag[], void>({
      query: () => '/tag/getAll',
      transformResponse(response: ServerInterface<serverTag[]>) {
        let transformed: ITag[] = []
        response.t.forEach((el) => {
          transformed.push({id: el.tagId, name: el.tagName***REMOVED***)
***REMOVED***)
        return transformed
  ***REMOVED***,
      providesTags: ['tag']
***REMOVED***),
    insertTag: builder.mutation<ITag, string>({
      query: (tag_name: string) => ({
        url: '/tag/insert',
        method: 'POST',
        body: {
          tagName: tag_name
***REMOVED***
  ***REMOVED***),
      invalidatesTags: ['tag']
***REMOVED***)
  ***REMOVED***)
***REMOVED***)

export const {
  useGetAllTagsQuery,
  useInsertTagMutation
***REMOVED*** = apiSlice