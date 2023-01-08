import {apiSlice***REMOVED*** from "./apiSlice";
import { ServerInterface ***REMOVED*** from "../types/serverInterface";
import { serverAlbum ***REMOVED*** from "../types/albumInterface";

const extendedAlbumApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getAllAlbums: builder.query<serverAlbum[], void>({
      query: () => '/gallery/getAll',
      transformResponse: (response: ServerInterface<serverAlbum[]>) => response.t,
      providesTags: ['album']
***REMOVED***)
  ***REMOVED***)
***REMOVED***)

export const {
  useGetAllAlbumsQuery
***REMOVED*** = extendedAlbumApi