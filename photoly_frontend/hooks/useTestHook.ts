import { useGetChildrenQuery, useGetRootQuery } from "../redux/api/namespaceSlice";
import { skipToken } from "@reduxjs/toolkit/query";

export default function useTestHook() {
  const {data: root, isSuccess} = useGetRootQuery()
  const {data: children} = useGetChildrenQuery(isSuccess ? root.nsId : skipToken)
  return null;
}