
import qs from "qs"
import { createContext, Dispatch, ReactNode, useContext, useReducer, useState } from "react"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"

interface ISearchFilter {
    tagIds: number[]
    albumId: number
}

interface IPhoto {
    id: number
    name: string
    format: string
    uploaddate: string
    visibility: boolean
}

interface IFolder {
    id: number
    name: string
    parentId: number
    userId: number
}

interface ISearchData {
    path: IFolder[]
    current: IFolder
    folders: IFolder[]
    photos: IPhoto[]
}

type ISearchFilterAction = 
    | {type: "add_tag" | "remove_tag" | "set_album" | "set_namespace", payload: number}
    | {type: "clear_album"}

const initSearchFilterState: ISearchFilter = {
    tagIds: [],
    albumId: -1
}

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

const searchFilterReducer = (state: ISearchFilter, action: ISearchFilterAction): ISearchFilter => {
    switch (action.type) {
        case "add_tag":
            if (!state.tagIds.includes(action.payload))
                return {tagIds: [...state.tagIds, action.payload], albumId: -1}
            return {tagIds: state.tagIds, albumId: -1}

        case "remove_tag":
            if (state.tagIds.includes(action.payload))
                return {tagIds: state.tagIds.filter(id => id !== action.payload), albumId: -1}
            return {tagIds: state.tagIds, albumId: -1}

        case "set_album":
            return {tagIds: [], albumId: action.payload}

        case "clear_album":
            return {tagIds: [], albumId: -1}

        default:
            throw new Error("Undefined action type")
    }
}

const SearchContext = createContext(initSearchFilterState)
const SearchDataContext = createContext(initSearchDataState)
const OpenFolderContext = createContext<(e: number) => void>(() => {})
const SearchContextUpdate = createContext<Dispatch<ISearchFilterAction>>(() => {})

export const useSearchFilter = () => useContext(SearchContext)
export const useSearchData = () => useContext(SearchDataContext)
export const useOpenFolder = () => useContext(OpenFolderContext)
export const useSearchUpdateDispatch = () => useContext(SearchContextUpdate)

const SearchContextProvider = ({children}: {children: ReactNode}) => {
    const [searchData, setSearchData] = useState(initSearchDataState)
    const [searchFilter, searchUpdateDispatch] = useReducer(searchFilterReducer, initSearchFilterState)
    const token = useToken()
    const { get } = useApi()

    // const setOnlyPhotos = (data: any) => {
    //     let photos: IPhoto[] = []
    //
    //     data.forEach((ele: any) => {
    //         photos.push({
    //             id: ele.photoId,
    //             name: ele.photoName,
    //             format: ele.format,
    //             uploaddate: ele.uploadDate,
    //             visibility: ele.visibility
    //         })
    //     })
    //     setSearchData({photos, path: [defaultFolder], current: defaultFolder, folders: []})
    // }

    // const searchByTags = async () => {
    //     const response = await get("/photo/getByTags", {
    //         headers: { "HRD-token": token },
    //         params: {
    //             tagIds: searchFilter.tagIds
    //         },
    //         paramsSerializer: params => qs.stringify(params, {arrayFormat: "comma", encode: false})
    //     })
    //
    //     if (!!response && response.data && response.data.msgCode === 200)
    //         setOnlyPhotos(response.data.t)
    // }

    // const searchByAlbum = async () => {
    //     const response = await get("/photo/getByGallery", {
    //         headers: { "HRD-token": token },
    //         params: {
    //             gaId: searchFilter.albumId
    //         },
    //         paramsSerializer: params => qs.stringify(params)
    //     })
    //
    //     if (!!response && response.data && response.data.msgCode === 200)
    //         setOnlyPhotos(response.data.t)
    // }

    const getRoot = async () => {
        const root = await get("/namespace/getRoot", {
            headers: { "HRD-token": token }
        })
        
        if (!!root && root.data && root.data.msgCode === 200) {
            const rootData = root.data.t
            let rootFolder: IFolder = {
                id: rootData.nsId,
                name: rootData.nsName,
                parentId: rootData.nsParentId,
                userId: rootData.userId
            }

            const [photos, folders] = await Promise.all([
                get("/photo/getByNamespace", {
                    headers: { "HRD-token": token },
                    params: {
                        nsId: rootFolder.id
                    },
                    paramsSerializer: params => qs.stringify(params)
                }),
                get("/namespace/getChildren", {
                    headers: { "HRD-token": token },
                    params: {
                        parentId: rootFolder.id
                    },
                    paramsSerializer: params => qs.stringify(params)
                })
            ])

            if (!!photos && !!folders && photos.data && folders.data && photos.data.msgCode === 200 && folders.data.msgCode === 200) {
                const photoData = photos.data.t
                const folderData = folders.data.t
                let photoArray: IPhoto[] = []
                let folderArray: IFolder[] = []

                photoData.forEach((ele: any) => {
                    photoArray.push({
                        id: ele.photoId,
                        name: ele.photoName,
                        format: ele.format,
                        uploaddate: ele.uploadDate,
                        visibility: ele.visibility
                    })
                })
                folderData.forEach((ele: any) => {
                    folderArray.push({
                        id: ele.nsId,
                        name: ele.nsName,
                        parentId: ele.nsParentId,
                        userId: ele.userId
                    })
                })

                setSearchData({path: [rootFolder], current: rootFolder, photos: photoArray, folders: folderArray})
            }
        }
    }

    // useDebounce(
    //     () => {
    //         if (!!token) {
    //             if (searchFilter.tagIds.length !== 0) {
    //                 searchByTags()
    //             } else if (searchFilter.albumId !== -1) {
    //                 searchByAlbum()
    //             } else {
    //                 getRoot()
    //             }
    //         }
    //     },
    //     500,
    //     [searchFilter, token]
    // )

    const openFolder = async (folderId: number) => {
        const [photos, folders] = await Promise.all([
            get("/photo/getByNamespace", {
                headers: { "HRD-token": token },
                params: {
                    nsId: folderId
                },
                paramsSerializer: params => qs.stringify(params)
            }),
            get("/namespace/getChildren", {
                headers: { "HRD-token": token },
                params: {
                    parentId: folderId
                },
                paramsSerializer: params => qs.stringify(params)
            })
        ])
        
        if (!!photos && !!folders && photos.data && folders.data && photos.data.msgCode === 200 && folders.data.msgCode === 200) {
            const photoData = photos.data.t
            const folderData = folders.data.t
            let photoArray: IPhoto[] = []
            let folderArray: IFolder[] = []

            photoData.forEach((ele: any) => {
                photoArray.push({
                    id: ele.photoId,
                    name: ele.photoName,
                    format: ele.format,
                    uploaddate: ele.uploadDate,
                    visibility: ele.visibility
                })
            })
            folderData.forEach((ele: any) => {
                folderArray.push({
                    id: ele.nsId,
                    name: ele.nsName,
                    parentId: ele.nsParentId,
                    userId: ele.userId
                })
            })

            let newCurr: IFolder = searchData.folders.find(f => f.id === folderId) || defaultFolder
            if (newCurr === defaultFolder) {
                getRoot()
                return
            } else {
                setSearchData(prev => ({
                    path: [...prev.path, newCurr],
                    current: newCurr,
                    photos: photoArray, 
                    folders: folderArray
                }))
            }
        }
    }
    
    return (
        <SearchContext.Provider value={searchFilter}>
            <SearchDataContext.Provider value={searchData}>
                <OpenFolderContext.Provider value={openFolder}>
                    <SearchContextUpdate.Provider value={searchUpdateDispatch}>
                        {children}
                    </SearchContextUpdate.Provider>
                </OpenFolderContext.Provider>
            </SearchDataContext.Provider>
        </SearchContext.Provider>
    )
}

export default SearchContextProvider