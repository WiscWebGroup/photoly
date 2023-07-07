import {createSlice} from "@reduxjs/toolkit";
import type {PayloadAction} from "@reduxjs/toolkit";
import { IFolder, ISearchData } from "../types/mainDisplayInterface";

const defaultFolder: IFolder = {
  id: -1,
  name: "/",
  parentId: -1,
  userId: -1
}

const initSearchDataState: ISearchData = {
  path: [defaultFolder],
  current: defaultFolder,
  folders: [],
  photos: []
}

export const mainDisplaySlice = createSlice({
  name: 'mainDisplay',
  initialState: initSearchDataState,
  reducers: {
    changeCurrDisplay: (state, action: PayloadAction<ISearchData>) => {
      state.path = action.payload.path
      state.current = action.payload.current
      state.folders = action.payload.folders
      state.photos = action.payload.photos
    }
  }
})

export const {
  changeCurrDisplay
} = mainDisplaySlice.actions
export default mainDisplaySlice.reducer