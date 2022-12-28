import {configureStore***REMOVED*** from "@reduxjs/toolkit";
import {apiSlice***REMOVED*** from "./api/apiSlice";
import searchFilterReducer from './states/searchFilterSlice'

const store = configureStore({
  reducer:{
    searchFilter: searchFilterReducer,
    [apiSlice.reducerPath]: apiSlice.reducer,
  ***REMOVED***,
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(apiSlice.middleware),
***REMOVED***)

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
export default store