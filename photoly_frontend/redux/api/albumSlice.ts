import {apiSlice***REMOVED*** from "./apiSlice";
import { ServerInterface ***REMOVED*** from "../types/serverInterface";
import { IAlbum, serverAlbum ***REMOVED*** from "../types/albumInterface";

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
***REMOVED***),
    deleteAlbum: builder.mutation({
      query: (album_id: number) => ({
        url: `/gallery/delete?gaId=${album_id***REMOVED***`,
        method: 'POST'
  ***REMOVED***),
      invalidatesTags: ['album']
***REMOVED***),
    renameAlbum: builder.mutation({
      query: ({id, name, coverId, coverColor***REMOVED***: IAlbum) => ({
        url: '/gallery/update',
        method: 'POST',
        body: {
          gaId: id,
          gaName: name,
          coverId,
          coverColor
***REMOVED***
  ***REMOVED***),
      invalidatesTags: ['album']
***REMOVED***)
  ***REMOVED***)
***REMOVED***)

export const {
  useGetAllAlbumsQuery,
  useInsertAlbumMutation,
  useDeleteAlbumMutation,
  useRenameAlbumMutation
***REMOVED*** = extendedAlbumApi