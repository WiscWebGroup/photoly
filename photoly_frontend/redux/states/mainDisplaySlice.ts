import {createSlice***REMOVED*** from "@reduxjs/toolkit";
import type {PayloadAction***REMOVED*** from "@reduxjs/toolkit";
import { IFolder, ISearchData ***REMOVED*** from "../types/mainDisplayInterface";

const defaultFolder: IFolder = {
  id: -1,
  name: "/",
  parentId: -1,
  userId: -1
***REMOVED***

const initSearchDataState: ISearchData = {
  path: [defaultFolder],
  current: defaultFolder,
  folders: [],
  photos: []
***REMOVED***

export const mainDisplaySlice = createSlice({
  name: 'mainDisplay',
  initialState: initSearchDataState,
  reducers: {
    changeCurrDisplay: (state, action: PayloadAction<ISearchData>) => {
      state.path = action.payload.path
      state.current = action.payload.current
      state.folders = action.payload.folders
      state.photos = action.payload.photos
***REMOVED***
  ***REMOVED***
***REMOVED***)

export const {
  changeCurrDisplay
***REMOVED*** = mainDisplaySlice.actions
export default mainDisplaySlice.reducer