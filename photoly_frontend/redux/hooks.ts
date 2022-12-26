import {TypedUseSelectorHook, useDispatch, useSelector***REMOVED*** from "react-redux";
import type {RootState, AppDispatch***REMOVED*** from "./store";

export const useAppDispatch: () => AppDispatch = useDispatch
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector