import { createContext, ReactNode, useContext, useEffect, useState ***REMOVED*** from "react"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"

interface IAlbum {
    id: number
    name: string
    coverId: number
    coverColor: string
***REMOVED***

interface IAlbumList {
    albums: IAlbum[]
***REMOVED***

interface IAlbumListUpdate {
    insertRequest: (name: string, coverId: number, coverColor: string) => void
    updateRequest: (e: IAlbum) => void
    deleteRequest: (id: number) => void
***REMOVED***

const initAlbumListState: IAlbumList = {
    albums: []
***REMOVED***

const AlbumContext = createContext(initAlbumListState)
const AlbumContextUpdate = createContext<IAlbumListUpdate>({
    insertRequest: () => {***REMOVED***,
    updateRequest: () => {***REMOVED***,
    deleteRequest: () => {***REMOVED***
***REMOVED***)

export const useAlbumList = () => useContext(AlbumContext)
export const useAlbumListUpdate = () => useContext(AlbumContextUpdate)

const AlbumContextProvider = ({ children ***REMOVED*** : { children?: ReactNode ***REMOVED***) => {
    const token = useToken()
    const { get, post ***REMOVED*** = useApi()
    const [albumList, setAlbumList] = useState(initAlbumListState)

    const getAlbums = async () => {
        const response = await get("/gallery/getAll", {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)
        if (!!response && response.data && response.data.msgCode === 200) {
            const data = response.data.t
            let albums: IAlbum[] = []

            data.forEach((ele: any) => {
                albums.push({
                    id: ele.gaId,
                    name: ele.gaName,
                    coverId: ele.coverId,
                    coverColor: ele.coverColor
   ***REMOVED*****REMOVED***)
    ***REMOVED***)

            setAlbumList({albums***REMOVED***)
***REMOVED***
***REMOVED***

    useEffect(() => {
        if (!!token) {
            getAlbums()
***REMOVED***
***REMOVED***, [token])

    const insertRequest = async (name: string, coverId: number, coverColor: string) => {
        const response = await post("/gallery/insert", {
            gaName: name,
            coverId,
            coverColor
***REMOVED***, {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200) {
            getAlbums()
***REMOVED***
***REMOVED***

    const updateRequest = async ({ id, name, coverId, coverColor***REMOVED*** : IAlbum) => {
        const response = await post("/gallery/update", {
            gaId: id,
            gaName: name,
            coverId,
            coverColor
***REMOVED***, {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200) {
            getAlbums()
***REMOVED***
***REMOVED***

    const deleteRequest = async (id: number) => {
        const response = await post("/gallery/delete", null, {
            headers: { "HRD-token": token ***REMOVED***,
            params: {
                gaId: id
    ***REMOVED***
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200) {
            getAlbums()
***REMOVED***
***REMOVED***

    return (
        <AlbumContext.Provider value={albumList***REMOVED***>
            <AlbumContextUpdate.Provider value={{insertRequest, updateRequest, deleteRequest***REMOVED******REMOVED***>
            ***REMOVED***children***REMOVED***
            </AlbumContextUpdate.Provider>
        </AlbumContext.Provider>
    )
***REMOVED***

export default AlbumContextProvider