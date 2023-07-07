import {configureStore} from "@reduxjs/toolkit";
import {apiSlice} from "./api/apiSlice";
import searchFilterReducer from './states/searchFilterSlice'
import mainDisplayReducer from "./states/mainDisplaySlice";

const store = configureStore({
  reducer:{
    mainDisplay: mainDisplayReducer,
    searchFilter: searchFilterReducer,
    [apiSlice.reducerPath]: apiSlice.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(apiSlice.middleware),
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
export default store