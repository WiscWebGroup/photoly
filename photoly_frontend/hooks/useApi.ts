import axios, {AxiosRequestHeaders, AxiosResponse} from "axios";
import useLocalStorage, {TOKEN_KEY} from "./useLocalStorage";
import {useRouter} from "next/router";
import {useToast} from "@chakra-ui/react";

export type TData = { [key: string]: any } | null
export type THeader = AxiosRequestHeaders | undefined

const defaultHeaders = {
    "Content-Type": "application/json",
    Accept: "application/json"
}

function useResponseHandler() {
    const {remove} = useLocalStorage(TOKEN_KEY)
    const router = useRouter()
    const toast = useToast()
    return ((res: AxiosResponse, path: string) => {
        if (res.status === 200) {
            // If request successful, check msgCode
            if (res.data.msgCode === 401) {
                toast({title: res.data.t, status: "warning", isClosable: true, position: "top"})
            } else if (res.data.msgCode == 403) {
                toast({title: "Unauthorized", status: "warning", isClosable: true, position: "top"})
                remove()
                router.push("/login")
                console.debug("Unauthenticated access")
            }
            return res
        } else if (res.status === 404) {
            toast({title: "404 Not Found", status: "warning", isClosable: true, position: "top"})
            console.debug(path)
            console.debug("Endpoint " + path + " was not found")
            return res
        } else if (res.status >= 500) {
            toast({title: "Sever Error", status: "error", isClosable: true, position: "top"})
            console.debug("Server error")
            return res
        } else {
            return res
        }
    })
}

export function useApi() {
    const handler = useResponseHandler()
    return {
        get: async (path: string, headers?: THeader): Promise<AxiosResponse> =>
            await axios.get(path, {
                headers: headers
            }).then(res => {
                return handler(res, path)
            }),
        post: async (path: string, data: TData, headers?: THeader): Promise<AxiosResponse> =>
            await axios.post(path, data, {
                headers: headers
            }).then(res => {
                return handler(res, path)
            }),
        put: async (path: string, data: TData, headers?: THeader): Promise<AxiosResponse> =>
            await axios.put(path, data, {
                headers: headers
            }).then(res => {
                return handler(res, path)
            }),
        del: async (path: string, headers?: THeader): Promise<AxiosResponse> =>
            await axios.delete(path, {
                headers: headers
            }).then(res => {
                return handler(res, path)
            }),
    }
}

export default useApi
