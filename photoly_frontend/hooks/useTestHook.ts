import { useGetChildrenQuery, useGetRootQuery ***REMOVED*** from "../redux/api/namespaceSlice";
import { skipToken ***REMOVED*** from "@reduxjs/toolkit/query";

export default function useTestHook() {
  const {data: root, isSuccess***REMOVED*** = useGetRootQuery()
  const {data: children***REMOVED*** = useGetChildrenQuery(isSuccess ? root.nsId : skipToken)
  return null;
***REMOVED***