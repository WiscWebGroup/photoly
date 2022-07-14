import { createContext, ReactNode, useContext, useEffect, useState ***REMOVED*** from "react"

interface ITag {
    tagId: number
    tagName: string
***REMOVED***

interface ITagList {
    tags: ITag[]
***REMOVED***

interface ITagListUpdate {
    renameRequest: (e: ITag) => void
    deleteRequest: (e: number) => void
***REMOVED***

const initTagListState: ITagList = {
    tags: []
***REMOVED***

const TagContext = createContext(initTagListState)
const TagContextUpdate = createContext<ITagListUpdate>({renameRequest: () => {***REMOVED***, deleteRequest: () => {***REMOVED******REMOVED***)

export const useTagList = () => useContext(TagContext)
export const useTagListUpdate = () => useContext(TagContextUpdate)

const TagContextProvider = ({ children ***REMOVED***: { children?: ReactNode ***REMOVED***) => {
    const [tagList, setTagList] = useState(initTagListState)

    useEffect(() => {
        // TODO: async api call to get the tag list
        // TODO: sort tag list in alphabetical ordering
***REMOVED***, [])

    const renameRequest = ({ tagId, tagName ***REMOVED***: ITag) => {
        // TODO: async api call to update ${tagId***REMOVED*** with new ${tagName***REMOVED***
        console.log(`The tag with id ${tagId***REMOVED*** is updated with the new name ${tagName***REMOVED***`)
***REMOVED***

    const deleteRequest = (tagId: number) => {
        // TODO: async api call to delete the tag with id ${tagId***REMOVED***
***REMOVED***

    return (
        <TagContext.Provider value={tagList***REMOVED***>
            <TagContextUpdate.Provider value={{renameRequest, deleteRequest***REMOVED******REMOVED***>
  ***REMOVED*****REMOVED*****REMOVED***children***REMOVED***
            </TagContextUpdate.Provider>
        </TagContext.Provider>
    )
***REMOVED***

export default TagContextProvider