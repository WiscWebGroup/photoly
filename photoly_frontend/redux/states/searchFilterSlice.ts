import {createSlice***REMOVED*** from "@reduxjs/toolkit";
import type {PayloadAction***REMOVED*** from "@reduxjs/toolkit";

export interface ISearchFilter {
  tagIds: number[]
  albumId: number
  folderId: number
***REMOVED***

const initSearchFilterState: ISearchFilter = {
  tagIds: [],
  albumId: -1,
  folderId: -1 // If this is -1, it means to query user's root folder
***REMOVED***

export const searchFilterSlice = createSlice({
  name: 'searchFilter',
  initialState: initSearchFilterState,
  reducers: {
    addTag: (state, action: PayloadAction<number>) => {
      state.tagIds.push(action.payload)
      state.albumId = -1
***REMOVED***,
    removeTag: (state, action: PayloadAction<number>) => {
      state.tagIds = state.tagIds.filter(id => id !== action.payload)
      state.albumId = -1
***REMOVED***,
    setAlbum: (state, action: PayloadAction<number>) => {
      state.tagIds = []
      state.albumId = action.payload
***REMOVED***,
    clearAlbum: (state) => {
      state.tagIds = []
      state.albumId = -1
***REMOVED***,
    setFolder: (state, action: PayloadAction<number>) => {
      state.folderId = action.payload
      state.tagIds = []
      state.albumId = -1
***REMOVED***,
    clearAllFilter: (state) => {
      state.folderId = -1
      state.tagIds = []
      state.albumId = -1
***REMOVED***
  ***REMOVED***
***REMOVED***)

export const {
  addTag,
  removeTag,
  setAlbum,
  clearAlbum,
  setFolder,
  clearAllFilter
***REMOVED*** = searchFilterSlice.actions
export default searchFilterSlice.reducer