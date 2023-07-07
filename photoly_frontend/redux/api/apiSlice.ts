import {createApi} from "@reduxjs/toolkit/query/react";
import { myBaseQuery } from "../customBaseQuery";

export const apiSlice = createApi({
  reducerPath: 'api',
  baseQuery: myBaseQuery,
  tagTypes: ['tag', 'album'],
  endpoints: () => ({})
})

