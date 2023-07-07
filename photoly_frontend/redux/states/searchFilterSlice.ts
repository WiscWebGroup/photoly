import {createSlice} from "@reduxjs/toolkit";
import type {PayloadAction} from "@reduxjs/toolkit";

export interface ISearchFilter {
  tagIds: number[]
  albumId: number
  folderId: number
}

const initSearchFilterState: ISearchFilter = {
  tagIds: [],
  albumId: -1,
  folderId: -1 // If this is -1, it means to query user's root folder
}

export const searchFilterSlice = createSlice({
  name: 'searchFilter',
  initialState: initSearchFilterState,
  reducers: {
    addTag: (state, action: PayloadAction<number>) => {
      state.tagIds.push(action.payload)
      state.albumId = -1
    },
    removeTag: (state, action: PayloadAction<number>) => {
      state.tagIds = state.tagIds.filter(id => id !== action.payload)
      state.albumId = -1
    },
    setAlbum: (state, action: PayloadAction<number>) => {
      state.tagIds = []
      state.albumId = action.payload
    },
    clearAlbum: (state) => {
      state.tagIds = []
      state.albumId = -1
    },
    setFolder: (state, action: PayloadAction<number>) => {
      state.folderId = action.payload
      state.tagIds = []
      state.albumId = -1
    },
    clearAllFilter: (state) => {
      state.folderId = -1
      state.tagIds = []
      state.albumId = -1
    }
  }
})

export const {
  addTag,
  removeTag,
  setAlbum,
  clearAlbum,
  setFolder,
  clearAllFilter
} = searchFilterSlice.actions
export default searchFilterSlice.reducer