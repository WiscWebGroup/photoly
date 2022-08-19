
import qs from "qs"
import { createContext, Dispatch, ReactNode, useContext, useReducer, useState ***REMOVED*** from "react"
import useApi from "../../hooks/useApi"
import useDebounce from "../../hooks/useDebounce"
import useToken from "../../hooks/useToken"

interface ISearchFilter {
    tagIds: number[]
    albumId: number
***REMOVED***

interface IPhoto {
    id: number
    name: string
    format: string
    uploaddate: string
    visibility: boolean
***REMOVED***

interface IFolder {
    id: number
    name: string
    parentId: number
    userId: number
***REMOVED***

interface ISearchData {
    path: IFolder[]
    current: IFolder
    folders: IFolder[]
    photos: IPhoto[]
***REMOVED***

type ISearchFilterAction = 
    | {type: "add_tag" | "remove_tag" | "set_album" | "set_namespace", payload: number***REMOVED***
    | {type: "clear_album"***REMOVED***

const initSearchFilterState: ISearchFilter = {
    tagIds: [],
    albumId: -1
***REMOVED***

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

const searchFilterReducer = (state: ISearchFilter, action: ISearchFilterAction): ISearchFilter => {
    switch (action.type) {
        case "add_tag":
            if (!state.tagIds.includes(action.payload))
                return {tagIds: [...state.tagIds, action.payload], albumId: -1***REMOVED***
            return {tagIds: state.tagIds, albumId: -1***REMOVED***

        case "remove_tag":
            if (state.tagIds.includes(action.payload))
                return {tagIds: state.tagIds.filter(id => id !== action.payload), albumId: -1***REMOVED***
            return {tagIds: state.tagIds, albumId: -1***REMOVED***

        case "set_album":
            return {tagIds: [], albumId: action.payload***REMOVED***

        case "clear_album":
            return {tagIds: [], albumId: -1***REMOVED***

        default:
            throw new Error("Undefined action type")
***REMOVED***
***REMOVED***

const SearchContext = createContext(initSearchFilterState)
const SearchDataContext = createContext(initSearchDataState)
const OpenFolderContext = createContext<(e: number) => void>(() => {***REMOVED***)
const SearchContextUpdate = createContext<Dispatch<ISearchFilterAction>>(() => {***REMOVED***)

export const useSearchFilter = () => useContext(SearchContext)
export const useSearchData = () => useContext(SearchDataContext)
export const useOpenFolder = () => useContext(OpenFolderContext)
export const useSearchUpdateDispatch = () => useContext(SearchContextUpdate)

const SearchContextProvider = ({children***REMOVED***: {children: ReactNode***REMOVED***) => {
    const [searchData, setSearchData] = useState(initSearchDataState)
    const [searchFilter, searchUpdateDispatch] = useReducer(searchFilterReducer, initSearchFilterState)
    const token = useToken()
    const { get ***REMOVED*** = useApi()

    const setOnlyPhotos = (data: any) => {
        let photos: IPhoto[] = []

        data.forEach((ele: any) => {
            photos.push({
                id: ele.photoId,
                name: ele.photoName,
                format: ele.format,
                uploaddate: ele.uploadDate,
                visibility: ele.visibility
    ***REMOVED***)
***REMOVED***)
        setSearchData({photos, path: [defaultFolder], current: defaultFolder, folders: []***REMOVED***)
***REMOVED***

    const searchByTags = async () => {
        const response = await get("/photo/getByTags", {
            headers: { "HRD-token": token ***REMOVED***,
            params: {
                tagIds: searchFilter.tagIds
    ***REMOVED***,
            paramsSerializer: params => qs.stringify(params, {arrayFormat: "comma", encode: false***REMOVED***)
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200)
            setOnlyPhotos(response.data.t)
***REMOVED***

    const searchByAlbum = async () => {
        const response = await get("/photo/getByGallery", {
            headers: { "HRD-token": token ***REMOVED***,
            params: {
                gaId: searchFilter.albumId
    ***REMOVED***,
            paramsSerializer: params => qs.stringify(params)
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200)
            setOnlyPhotos(response.data.t)
***REMOVED***

    const getRoot = async () => {
        const root = await get("/namespace/getRoot", {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)
        
        if (!!root && root.data && root.data.msgCode === 200) {
            const rootData = root.data.t
            let rootFolder: IFolder = {
                id: rootData.nsId,
                name: rootData.nsName,
                parentId: rootData.nsParentId,
                userId: rootData.userId
    ***REMOVED***

            const [photos, folders] = await Promise.all([
                get("/photo/getByNamespace", {
                    headers: { "HRD-token": token ***REMOVED***,
                    params: {
                        nsId: rootFolder.id
  ***REMOVED*****REMOVED*****REMOVED***,
                    paramsSerializer: params => qs.stringify(params)
   ***REMOVED*****REMOVED***),
                get("/namespace/getChildren", {
                    headers: { "HRD-token": token ***REMOVED***,
                    params: {
                        parentId: rootFolder.id
  ***REMOVED*****REMOVED*****REMOVED***,
                    paramsSerializer: params => qs.stringify(params)
   ***REMOVED*****REMOVED***)
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
  ***REMOVED*****REMOVED*****REMOVED***)
   ***REMOVED*****REMOVED***)
                folderData.forEach((ele: any) => {
                    folderArray.push({
                        id: ele.nsId,
                        name: ele.nsName,
                        parentId: ele.nsParentId,
                        userId: ele.userId
  ***REMOVED*****REMOVED*****REMOVED***)
   ***REMOVED*****REMOVED***)

                setSearchData({path: [rootFolder], current: rootFolder, photos: photoArray, folders: folderArray***REMOVED***)
    ***REMOVED***
***REMOVED***
***REMOVED***

    useDebounce(
        () => {
            if (!!token) {
                if (searchFilter.tagIds.length !== 0) {
                    searchByTags()
   ***REMOVED*****REMOVED*** else if (searchFilter.albumId !== -1) {
                    searchByAlbum()
   ***REMOVED*****REMOVED*** else {
                    getRoot()
   ***REMOVED*****REMOVED***
    ***REMOVED***
***REMOVED***,
        500,
        [searchFilter, token]
    )

    const openFolder = async (folderId: number) => {
        const [photos, folders] = await Promise.all([
            get("/photo/getByNamespace", {
                headers: { "HRD-token": token ***REMOVED***,
                params: {
                    nsId: folderId
   ***REMOVED*****REMOVED***,
                paramsSerializer: params => qs.stringify(params)
    ***REMOVED***),
            get("/namespace/getChildren", {
                headers: { "HRD-token": token ***REMOVED***,
                params: {
                    parentId: folderId
   ***REMOVED*****REMOVED***,
                paramsSerializer: params => qs.stringify(params)
    ***REMOVED***)
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
   ***REMOVED*****REMOVED***)
    ***REMOVED***)
            folderData.forEach((ele: any) => {
                folderArray.push({
                    id: ele.nsId,
                    name: ele.nsName,
                    parentId: ele.nsParentId,
                    userId: ele.userId
   ***REMOVED*****REMOVED***)
    ***REMOVED***)

            let newCurr: IFolder = searchData.folders.find(f => f.id === folderId) || defaultFolder
            if (newCurr === defaultFolder) {
                getRoot()
                return
    ***REMOVED*** else {
                setSearchData(prev => ({
                    path: [...prev.path, newCurr],
                    current: newCurr,
                    photos: photoArray, 
                    folders: folderArray
   ***REMOVED*****REMOVED***))
    ***REMOVED***
***REMOVED***
***REMOVED***
    
    return (
        <SearchContext.Provider value={searchFilter***REMOVED***>
            <SearchDataContext.Provider value={searchData***REMOVED***>
                <OpenFolderContext.Provider value={openFolder***REMOVED***>
                    <SearchContextUpdate.Provider value={searchUpdateDispatch***REMOVED***>
     ***REMOVED*****REMOVED*****REMOVED*****REMOVED***children***REMOVED***
                    </SearchContextUpdate.Provider>
                </OpenFolderContext.Provider>
            </SearchDataContext.Provider>
        </SearchContext.Provider>
    )
***REMOVED***

export default SearchContextProvider