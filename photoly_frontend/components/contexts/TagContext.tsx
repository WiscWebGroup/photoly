import { createContext, ReactNode, useContext, useEffect, useState } from "react"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"

interface ITag {
    id: number
    name: string
}

interface ITagList {
    tags: ITag[]
}

interface ITagListUpdate {
    insertRequest: (e: string) => void
    renameRequest: (e: ITag) => void
    deleteRequest: (e: number) => void
}

const initTagListState: ITagList = {
    tags: []
}

const TagContext = createContext(initTagListState)
const TagContextUpdate = createContext<ITagListUpdate>({
    insertRequest: () => {},
    renameRequest: () => {},
    deleteRequest: () => {}
})

export const useTagList = () => useContext(TagContext)
export const useTagListUpdate = () => useContext(TagContextUpdate)

const TagContextProvider = ({ children }: { children?: ReactNode }) => {
    const token = useToken()
    const { get, post } = useApi()
    const [tagList, setTagList] = useState(initTagListState)

    const getTags = async () => {
        const response = await get("/tag/getAll", {
            headers: { "HRD-token": token },
        })
        if (!!response && response.data && response.data.msgCode == 200) {
            const data = response.data.t
            let tags: ITag[] = []

            data.forEach((ele: any) => {
                tags.push({ id: ele.tagId, name: ele.tagName })
            })

            setTagList({tags})
        }
    }

    useEffect(() => {
        if (!!token) {
            getTags()
        }
    }, [token])

    const insertRequest = async (name: string) => {
        const data = {tagName: name}
        const response = await post("/tag/insert", data, {
            headers: { "HRD-token": token }
        })
        if (!!response && response.data) {
            getTags()
        }
    }

    const renameRequest = async ({ id, name }: ITag) => {
        const data = {tagId: id, tagName: name}
        const response = await post("/tag/update", data, {
            headers: { "HRD-token": token }
        })
        if (!!response && response.data) {
            getTags()
        }
    }

    const deleteRequest = async (id: number) => {
        const response = await post(`/tag/delete?tagId=${id}`, {}, {
            headers: { "HRD-token": token }
        })
        if (!!response && response.data) {
            getTags()
        }
    }

    return (
        <TagContext.Provider value={tagList}>
            <TagContextUpdate.Provider value={{insertRequest, renameRequest, deleteRequest}}>
                {children}
            </TagContextUpdate.Provider>
        </TagContext.Provider>
    )
}

export default TagContextProvider