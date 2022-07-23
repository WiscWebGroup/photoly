import { createContext, ReactNode, useContext, useEffect, useState ***REMOVED*** from "react"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"

interface ITag {
    id: number
    name: string
***REMOVED***

interface ITagList {
    tags: ITag[]
***REMOVED***

interface ITagListUpdate {
    insertRequest: (e: string) => void
    renameRequest: (e: ITag) => void
    deleteRequest: (e: number) => void
***REMOVED***

const initTagListState: ITagList = {
    tags: []
***REMOVED***

const TagContext = createContext(initTagListState)
const TagContextUpdate = createContext<ITagListUpdate>({
    insertRequest: () => {***REMOVED***,
    renameRequest: () => {***REMOVED***,
    deleteRequest: () => {***REMOVED***
***REMOVED***)

export const useTagList = () => useContext(TagContext)
export const useTagListUpdate = () => useContext(TagContextUpdate)

const TagContextProvider = ({ children ***REMOVED***: { children?: ReactNode ***REMOVED***) => {
    const token = useToken()
    const { get, post ***REMOVED*** = useApi()
    const [tagList, setTagList] = useState(initTagListState)

    const getTags = async () => {
        const response = await get("/tag/getAll", {
            headers: { "HRD-token": token ***REMOVED***,
***REMOVED***)
        if (!!response && response.data) {
            const data = response.data.t
            let tags: ITag[] = []

            data.forEach((ele: any) => {
                tags.push({ id: ele.tagId, name: ele.tagName ***REMOVED***)
    ***REMOVED***)

            setTagList({tags***REMOVED***)
***REMOVED***
***REMOVED***

    useEffect(() => {
        if (!!token) {
            getTags()
***REMOVED***
***REMOVED***, [token])

    const insertRequest = async (name: string) => {
        const data = {tagName: name***REMOVED***
        const response = await post("/tag/insert", data, {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)
        if (!!response && response.data) {
            getTags()
***REMOVED***
***REMOVED***

    const renameRequest = async ({ id, name ***REMOVED***: ITag) => {
        const data = {tagId: id, tagName: name***REMOVED***
        const response = await post("/tag/update", data, {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)
        if (!!response && response.data) {
            getTags()
***REMOVED***
***REMOVED***

    const deleteRequest = async (id: number) => {
        const response = await post(`/tag/delete?tagId=${id***REMOVED***`, {***REMOVED***, {
            headers: { "HRD-token": token ***REMOVED***
***REMOVED***)
        if (!!response && response.data) {
            getTags()
***REMOVED***
***REMOVED***

    return (
        <TagContext.Provider value={tagList***REMOVED***>
            <TagContextUpdate.Provider value={{insertRequest, renameRequest, deleteRequest***REMOVED******REMOVED***>
            ***REMOVED***children***REMOVED***
            </TagContextUpdate.Provider>
        </TagContext.Provider>
    )
***REMOVED***

export default TagContextProvider