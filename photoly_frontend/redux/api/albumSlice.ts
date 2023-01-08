import {apiSlice***REMOVED*** from "./apiSlice";
import { ServerInterface ***REMOVED*** from "../types/serverInterface";
import { serverAlbum ***REMOVED*** from "../types/albumInterface";

const extendedAlbumApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getAllAlbums: builder.query<serverAlbum[], void>({
      query: () => '/gallery/getAll',
      transformResponse: (response: ServerInterface<serverAlbum[]>) => response.t,
      providesTags: ['album']
***REMOVED***),
    insertAlbum: builder.mutation({
      query: ({color, id, name***REMOVED***: {color: string, id: number, name: string***REMOVED***) => ({
        url: '/gallery/insert',
        method: 'POST',
        body: {
          coverColor: color,
          coverId: id,
          gaName: name
***REMOVED***
  ***REMOVED***),
      invalidatesTags: ['album']
***REMOVED***)
  ***REMOVED***)
***REMOVED***)

export const {
  useGetAllAlbumsQuery,
  useInsertAlbumMutation
***REMOVED*** = extendedAlbumApi