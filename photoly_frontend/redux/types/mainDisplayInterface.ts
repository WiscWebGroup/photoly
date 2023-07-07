export interface IPhoto {
  id: number
  name: string
  format: string
  uploaddate: string
  visibility: boolean
}

export interface IFolder {
  id: number
  name: string
  parentId: number
  userId: number
}

export interface ISearchData {
  path: IFolder[]
  current: IFolder
  folders: IFolder[]
  photos: IPhoto[]
}