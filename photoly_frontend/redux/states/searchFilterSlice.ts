import {createSlice***REMOVED*** from "@reduxjs/toolkit";
import type {PayloadAction***REMOVED*** from "@reduxjs/toolkit";

export interface ISearchFilter {
  tagIds: number[]
  albumId: number
***REMOVED***

const initSearchFilterState: ISearchFilter = {
  tagIds: [],
  albumId: -1
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
***REMOVED***
  ***REMOVED***
***REMOVED***)

export const {
  addTag,
  removeTag,
  setAlbum,
  clearAlbum
***REMOVED*** = searchFilterSlice.actions
export default searchFilterSlice.reducer