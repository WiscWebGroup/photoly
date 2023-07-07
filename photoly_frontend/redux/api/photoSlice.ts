import { apiSlice } from "./apiSlice";
import { ServerInterface } from "../types/serverInterface";
import { serverPhoto } from "../types/photoInterface";

const extendedPhotoApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getPhotoByNamespace: builder.query<serverPhoto[], number>({
      query: (ns_id: number) => `/photo/getByNamespace?nsId=${ns_id}`,
      transformResponse: (response: ServerInterface<serverPhoto[]>) => response.t
    }),
    getPhotoByTags: builder.query<serverPhoto[], number[]>({
      query: (tags: number[]) => {
        return `/photo/getByTags?tagIds=${tags.join(',')}`
      },
      transformResponse: (response: ServerInterface<serverPhoto[]>) => response.t
    }),
    getPhotoByAlbum: builder.query<serverPhoto[], number>({
      query: (album_id: number) => `/photo/getByGallery?gaId=${album_id}`,
      transformResponse: (response: ServerInterface<serverPhoto[]>) => response.t
    })
  })
})

export const {
  useGetPhotoByNamespaceQuery,
  useGetPhotoByTagsQuery,
  useGetPhotoByAlbumQuery
} = extendedPhotoApi