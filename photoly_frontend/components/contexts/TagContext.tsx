import { createContext, ReactNode, useContext, useEffect, useState } from "react"

interface ITag {
    tagId: number
    tagName: string
}

interface ITagList {
    tags: ITag[]
}

interface ITagListUpdate {
    renameRequest: (e: ITag) => void
    deleteRequest: (e: number) => void
}

const initTagListState: ITagList = {
    tags: []
}

const TagContext = createContext(initTagListState)
const TagContextUpdate = createContext<ITagListUpdate>({renameRequest: () => {}, deleteRequest: () => {}})

export const useTagList = () => useContext(TagContext)
export const useTagListUpdate = () => useContext(TagContextUpdate)

const TagContextProvider = ({ children }: { children?: ReactNode }) => {
    const [tagList, setTagList] = useState(initTagListState)

    useEffect(() => {
        // TODO: async api call to get the tag list
        // TODO: sort tag list in alphabetical ordering
    }, [])

    const renameRequest = ({ tagId, tagName }: ITag) => {
        // TODO: async api call to update ${tagId} with new ${tagName}
        console.log(`The tag with id ${tagId} is updated with the new name ${tagName}`)
    }

    const deleteRequest = (tagId: number) => {
        // TODO: async api call to delete the tag with id ${tagId}
    }

    return (
        <TagContext.Provider value={tagList}>
            <TagContextUpdate.Provider value={{renameRequest, deleteRequest}}>
                {children}
            </TagContextUpdate.Provider>
        </TagContext.Provider>
    )
}

export default TagContextProvider