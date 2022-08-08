import { createContext, ReactNode, useContext, useEffect, useState ***REMOVED*** from "react"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"

interface IPhoto {
    id: number
    name: string
    visibility: boolean
***REMOVED***

interface INamespace {
    id: number
    name: string
    parentId: number
    userId: number
***REMOVED***

interface IPanelData {
    current: INamespace
    children: INamespace[]
    photos: IPhoto[]
***REMOVED***

const initPanelData: IPanelData = {
    current: {
        id: -1,
        name: "/",
        parentId: -1,
        userId: -1
***REMOVED***,
    children: [],
    photos: []
***REMOVED***

const PanelDataContext = createContext(initPanelData)

export const usePanelDataContext = () => useContext(PanelDataContext)

const PanelContextProvider = ({ children ***REMOVED***: { children?: ReactNode ***REMOVED***) => {
    const token = useToken()
    const { get ***REMOVED*** = useApi()
    const [panelData, setPanelData] = useState(initPanelData)

    const getRootPanelData = async () => {
        const response = await get("/namespace/getRoot", {
            headers: { "HRD-token": token ***REMOVED***,
***REMOVED***)

        if (!!response && response.data && response.data.msgCode === 200) {
            const data = response.data.t

            setPanelData(prev => ({...prev, current: {
                id: data.nsId,
                name: data.nsName,
                parentId: data.nsParentId,
                userId: data.userId
    ***REMOVED******REMOVED***))

            getChildrenNamespaces(data.nsId)
            getPhotos(data.nsId)
***REMOVED***
***REMOVED***

    const getChildrenNamespaces = async (parentId: number) => {
        const response = await get(`/namespace/getChildren?parentId=${parentId***REMOVED***`, {
            headers: { "HRD-token": token ***REMOVED***,
***REMOVED***)

        if (!!response && response.data && response.data.msgCode == 200) {
            const data = response.data.t
            let namespaces: INamespace[] = []

            data.forEach((ele: any) => {
                namespaces.push({
                    id: ele.nsId,
                    name: ele.nsName,
                    parentId: ele.nsParentId,
                    userId: ele.userId
   ***REMOVED*****REMOVED***)
    ***REMOVED***)

            setPanelData(prev => ({...prev, children: namespaces***REMOVED***))
***REMOVED***
***REMOVED***

    const getPhotos = async (nsId: number) => {
        const response = await get(`/photo/getByNamespace?nsId=${nsId***REMOVED***`, {
            headers: { "HRD-token": token ***REMOVED***,
***REMOVED***)

        if (!!response && response.data && response.data.msgCode == 200) {
            const data = response.data.t
            let photos: IPhoto[] = []

            data.forEach((ele: any) => {
                photos.push({
                    id: ele.photoId,
                    name: ele.photoName,
                    visibility: ele.visibility
   ***REMOVED*****REMOVED***)
    ***REMOVED***)

            setPanelData(prev => ({...prev, photos***REMOVED***))
***REMOVED***
***REMOVED***

    useEffect(() => {
        if (!!token) {
            getRootPanelData()
***REMOVED***
***REMOVED***, [token])

    return (
        <PanelDataContext.Provider value={panelData***REMOVED***>
        ***REMOVED***children***REMOVED***
        </PanelDataContext.Provider>
    )
***REMOVED***

export default PanelContextProvider