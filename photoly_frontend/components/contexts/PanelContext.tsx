import { createContext, ReactNode, useContext, useEffect, useState } from "react"
import useApi from "../../hooks/useApi"
import useToken from "../../hooks/useToken"

interface IPhoto {
    id: number
    name: string
    visibility: boolean
}

interface INamespace {
    id: number
    name: string
    parentId: number
    userId: number
}

interface IPanelData {
    current: INamespace
    children: INamespace[]
    photos: IPhoto[]
}

const initPanelData: IPanelData = {
    current: {
        id: -1,
        name: "/",
        parentId: -1,
        userId: -1
    },
    children: [],
    photos: []
}

const PanelDataContext = createContext(initPanelData)

export const usePanelDataContext = () => useContext(PanelDataContext)

const PanelContextProvider = ({ children }: { children?: ReactNode }) => {
    const token = useToken()
    const { get } = useApi()
    const [panelData, setPanelData] = useState(initPanelData)

    const getRootPanelData = async () => {
        const response = await get("/namespace/getRoot", {
            headers: { "HRD-token": token },
        })

        if (!!response && response.data && response.data.msgCode === 200) {
            const data = response.data.t

            setPanelData(prev => ({...prev, current: {
                id: data.nsId,
                name: data.nsName,
                parentId: data.nsParentId,
                userId: data.userId
            }}))

            getChildrenNamespaces(data.nsId)
            getPhotos(data.nsId)
        }
    }

    const getChildrenNamespaces = async (parentId: number) => {
        const response = await get(`/namespace/getChildren?parentId=${parentId}`, {
            headers: { "HRD-token": token },
        })

        if (!!response && response.data && response.data.msgCode == 200) {
            const data = response.data.t
            let namespaces: INamespace[] = []

            data.forEach((ele: any) => {
                namespaces.push({
                    id: ele.nsId,
                    name: ele.nsName,
                    parentId: ele.nsParentId,
                    userId: ele.userId
                })
            })

            setPanelData(prev => ({...prev, children: namespaces}))
        }
    }

    const getPhotos = async (nsId: number) => {
        const response = await get(`/photo/getByNamespace?nsId=${nsId}`, {
            headers: { "HRD-token": token },
        })

        if (!!response && response.data && response.data.msgCode == 200) {
            const data = response.data.t
            let photos: IPhoto[] = []

            data.forEach((ele: any) => {
                photos.push({
                    id: ele.photoId,
                    name: ele.photoName,
                    visibility: ele.visibility
                })
            })

            setPanelData(prev => ({...prev, photos}))
        }
    }

    useEffect(() => {
        if (!!token) {
            getRootPanelData()
        }
    }, [token])

    return (
        <PanelDataContext.Provider value={panelData}>
            {children}
        </PanelDataContext.Provider>
    )
}

export default PanelContextProvider