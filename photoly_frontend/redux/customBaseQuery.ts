import type { BaseQueryFn, FetchArgs, FetchBaseQueryError } from "@reduxjs/toolkit/query";
import { fetchBaseQuery } from "@reduxjs/toolkit/query/react";
import { ServerInterface } from "./types/serverInterface";
import Router from "next/router";
import {createStandaloneToast} from "@chakra-ui/react";

const baseQuery = fetchBaseQuery({
  baseUrl: "",
  prepareHeaders: (headers) => {
    const token = localStorage.getItem("HRD-Token");
    if (!!token) {
      headers.set("HRD-token", token);
    }
    return headers;
  }
});

export const myBaseQuery: BaseQueryFn<string | FetchArgs,
  unknown,
  FetchBaseQueryError> = async (args, api, extraOptions) => {
  let result = await baseQuery(args, api, extraOptions)
  const {toast} = createStandaloneToast()
  if (result.error) {
    toast({
      title: 'Fatal server error',
      status: 'error',
      position: 'top'
    })
  }
  let cast = result.data as ServerInterface<any>
  switch (cast.msgCode){
    case 403:
      toast({
        title: 'Unauthorized access, redirecting...',
        status: "error",
        position: "top"
      })
      await Router.push('/login')
      return {error: {status: cast.msgCode, data: cast.t} }
    case 401:
    case 400:
      toast({
        title: 'Something is wrong...',
        status: "warning",
        position: "top"
      })
      return {error: {status: cast.msgCode, data: cast.t} }
  }
  // status 200
  return result;
};