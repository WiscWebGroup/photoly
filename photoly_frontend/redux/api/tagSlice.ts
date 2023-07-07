import {apiSlice} from "./apiSlice";
import { serverTag } from "../types/tagInterface";
import { ServerInterface } from "../types/serverInterface";

const extendedTagApi = apiSlice.injectEndpoints({
  endpoints: builder => ({
    getAllTags: builder.query<serverTag[], void>({
      query: () => '/tag/getAll',
      transformResponse(response: ServerInterface<serverTag[]>) {
        return response.t
      },
      providesTags: ['tag']
    }),
    insertTag: builder.mutation({
      query: (tag_name: string) => ({
        url: '/tag/insert',
        method: 'POST',
        body: {
          tagName: tag_name
        }
      }),
      invalidatesTags: ['tag']
    }),
    deleteTag: builder.mutation({
      query: (tag_id: number) => ({
        url: `/tag/delete?tagId=${tag_id}`,
        method: 'POST'
      }),
      invalidatesTags: ['tag']
    }),
    renameTag: builder.mutation({
      query: ({tag_id, new_name}: {tag_id: number, new_name: string}) => ({
        url: '/tag/update',
        method: 'POST',
        body: {
          tagId: tag_id,
          tagName: new_name
        }
      }),
      invalidatesTags: ['tag']
    })
  })
})

export const {
  useGetAllTagsQuery,
  useInsertTagMutation,
  useDeleteTagMutation,
  useRenameTagMutation
} = extendedTagApi