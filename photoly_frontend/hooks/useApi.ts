import axios, {AxiosError, AxiosRequestHeaders, AxiosResponse} from "axios";
import useLocalStorage, {TOKEN_KEY} from "./useLocalStorage";
import {useRouter} from "next/router";
import {useBoolean, useToast} from "@chakra-ui/react";

export type TData = { [key: string]: any } | null
export type THeader = AxiosRequestHeaders | undefined

const defaultHeaders = {
    "Content-Type": "application/json",
    Accept: "application/json"
}

// Handles status code == 200
function useResponseHandler() {
    const {remove} = useLocalStorage(TOKEN_KEY)
    const router = useRouter()
    const toast = useToast()
    return ((res: AxiosResponse) => {
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
    })
}

// Handles status code != 200
function useErrorHandler() {
    const toast = useToast()
    return ((err: AxiosError, path: string) => {
        if (!!err.response && err.response.status === 404) {
            toast({title: "404 Not Found", status: "warning", isClosable: true, position: "top"})
            console.debug(path)
            console.debug("Endpoint " + path + " was not found")
            return err
        } else if (!!err.response && err.response.status === 500) {
            toast({title: "Sever Error", status: "error", isClosable: true, position: "top"})
            console.debug("Server error")
            return err
        }
    })
}

export function useApi() {
    const respHandler = useResponseHandler()
    const errHandler = useErrorHandler()
    const [isLoading, setLoading] = useBoolean(false)
    return {
        isLoading,
        get: async (path: string, headers?: THeader): Promise<AxiosResponse | void> => {
            setLoading.on()
            return await axios.get(path, {
                headers: headers
            }).then(res => {
                setLoading.off()
                return respHandler(res)
            }).catch(err => {
                setLoading.off()
                errHandler(err, path)
            })
        },
        post: async (path: string, data: TData, headers?: THeader): Promise<AxiosResponse | void> => {
            setLoading.on()
            return await axios.post(path, data, {
                headers: headers
            }).then(res => {
                setLoading.off()
                return respHandler(res)
            }).catch(err => {
                setLoading.off()
                errHandler(err, path)
            })
        },
        put: async (path: string, data: TData, headers?: THeader): Promise<AxiosResponse | void> => {
            setLoading.on()
            return await axios.put(path, data, {
                headers: headers
            }).then(res => {
                setLoading.off()
                return respHandler(res)
            }).catch(err => {
                setLoading.off()
                errHandler(err, path)
            })
        },
        del: async (path: string, headers?: THeader): Promise<AxiosResponse | void> => {
            setLoading.on()
            return await axios.delete(path, {
                headers: headers
            }).then(res => {
                setLoading.off()
                return respHandler(res)
            }).catch(err => {
                setLoading.off()
                errHandler(err, path)
            })
        },
    }
}

export default useApi
