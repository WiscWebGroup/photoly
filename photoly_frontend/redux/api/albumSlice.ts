import {apiSlice} from "./apiSlice";
import { ServerInterface } from "../types/serverInterface";
import { IAlbum, serverAlbum } from "../types/albumInterface";

const extendedAlbumApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getAllAlbums: builder.query<serverAlbum[], void>({
      query: () => '/gallery/getAll',
      transformResponse: (response: ServerInterface<serverAlbum[]>) => response.t,
      providesTags: ['album']
    }),
    insertAlbum: builder.mutation({
      query: ({color, id, name}: {color: string, id: number, name: string}) => ({
        url: '/gallery/insert',
        method: 'POST',
        body: {
          coverColor: color,
          coverId: id,
          gaName: name
        }
      }),
      invalidatesTags: ['album']
    }),
    deleteAlbum: builder.mutation({
      query: (album_id: number) => ({
        url: `/gallery/delete?gaId=${album_id}`,
        method: 'POST'
      }),
      invalidatesTags: ['album']
    }),
    updateAlbum: builder.mutation({
      query: ({id, name, coverId, coverColor}: IAlbum) => ({
        url: '/gallery/update',
        method: 'POST',
        body: {
          gaId: id,
          gaName: name,
          coverId,
          coverColor
        }
      }),
      invalidatesTags: ['album']
    })
  })
})

export const {
  useGetAllAlbumsQuery,
  useInsertAlbumMutation,
  useDeleteAlbumMutation,
  useUpdateAlbumMutation
} = extendedAlbumApi