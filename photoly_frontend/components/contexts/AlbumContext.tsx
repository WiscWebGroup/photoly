import { createContext, ReactNode, useContext, useEffect, useState } from "react"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"

interface IAlbum {
    id: number
    name: string
    coverId: number
    coverColor: string
}

interface IAlbumList {
    albums: IAlbum[]
}

interface IAlbumListUpdate {
    insertRequest: (name: string, coverId: number, coverColor: string) => void
    updateRequest: (e: IAlbum) => void
    deleteRequest: (id: number) => void
}

const initAlbumListState: IAlbumList = {
    albums: []
}

const AlbumContext = createContext(initAlbumListState)
const AlbumContextUpdate = createContext<IAlbumListUpdate>({
    insertRequest: () => {},
    updateRequest: () => {},
    deleteRequest: () => {}
})

export const useAlbumList = () => useContext(AlbumContext)
export const useAlbumListUpdate = () => useContext(AlbumContextUpdate)

const AlbumContextProvider = ({ children } : { children?: ReactNode }) => {
    const token = useToken()
    const { get, post } = useApi()
    const [albumList, setAlbumList] = useState(initAlbumListState)

    const getAlbums = async () => {
        const response = await get("/gallery/getAll", {
            headers: { "HRD-token": token }
        })
        if (!!response && response.data && response.data.msgCode === 200) {
            const data = response.data.t
            let albums: IAlbum[] = []

            data.forEach((ele: any) => {
                albums.push({
                    id: ele.gaId,
                    name: ele.gaName,
                    coverId: ele.coverId,
                    coverColor: ele.coverColor
                })
            })

            setAlbumList({albums})
        }
    }

    useEffect(() => {
        if (!!token) {
            getAlbums()
        }
    }, [token])

    const insertRequest = async (name: string, coverId: number, coverColor: string) => {
        const response = await post("/gallery/insert", {
            gaName: name,
            coverId,
            coverColor
        }, {
            headers: { "HRD-token": token }
        })

        if (!!response && response.data && response.data.msgCode === 200) {
            getAlbums()
        }
    }

    const updateRequest = async ({ id, name, coverId, coverColor} : IAlbum) => {
        const response = await post("/gallery/update", {
            gaId: id,
            gaName: name,
            coverId,
            coverColor
        }, {
            headers: { "HRD-token": token }
        })

        if (!!response && response.data && response.data.msgCode === 200) {
            getAlbums()
        }
    }

    const deleteRequest = async (id: number) => {
        const response = await post("/gallery/delete", null, {
            headers: { "HRD-token": token },
            params: {
                gaId: id
            }
        })

        if (!!response && response.data && response.data.msgCode === 200) {
            getAlbums()
        }
    }

    return (
        <AlbumContext.Provider value={albumList}>
            <AlbumContextUpdate.Provider value={{insertRequest, updateRequest, deleteRequest}}>
                {children}
            </AlbumContextUpdate.Provider>
        </AlbumContext.Provider>
    )
}

export default AlbumContextProvider