import type { BaseQueryFn, FetchArgs, FetchBaseQueryError ***REMOVED*** from "@reduxjs/toolkit/query";
import { fetchBaseQuery ***REMOVED*** from "@reduxjs/toolkit/query";
import { ServerInterface ***REMOVED*** from "./types/serverInterface";
import Router from "next/router";
import {createStandaloneToast***REMOVED*** from "@chakra-ui/react";

const baseQuery = fetchBaseQuery({
  baseUrl: "",
  prepareHeaders: (headers) => {
    const token = localStorage.getItem("HRD-Token");
    if (!!token) {
      headers.set("HRD-token", token);
***REMOVED***
    return headers;
  ***REMOVED***
***REMOVED***);

export const myBaseQuery: BaseQueryFn<string | FetchArgs,
  unknown,
  FetchBaseQueryError> = async (args, api, extraOptions) => {
  let result = await baseQuery(args, api, extraOptions)
  const {toast***REMOVED*** = createStandaloneToast()
  if (result.error) {
    toast({
      title: 'Fatal server error',
      status: 'error',
      position: 'top'
***REMOVED***)
  ***REMOVED***
  let cast = result.data as ServerInterface<any>
  switch (cast.msgCode){
    case 403:
      toast({
        title: 'Unauthorized access, redirecting...',
        status: "error",
        position: "top"
  ***REMOVED***)
      await Router.push('/login')
      break
    case 401:
    case 400:
      toast({
        title: 'Something is wrong...',
        status: "warning",
        position: "top"
  ***REMOVED***)
  ***REMOVED***
  // status 200
  return result;
***REMOVED***;