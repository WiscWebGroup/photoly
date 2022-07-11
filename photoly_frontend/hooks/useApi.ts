import axios, {AxiosRequestHeaders, AxiosResponse***REMOVED*** from "axios";
import useLocalStorage, {TOKEN_KEY***REMOVED*** from "./useLocalStorage";
import {useRouter***REMOVED*** from "next/router";
import {useToast***REMOVED*** from "@chakra-ui/react";

export type TData = { [key: string]: any ***REMOVED*** | null
export type THeader = AxiosRequestHeaders | undefined

const defaultHeaders = {
    "Content-Type": "application/json",
    Accept: "application/json"
***REMOVED***

function useResponseHandler() {
    const {remove***REMOVED*** = useLocalStorage(TOKEN_KEY)
    const router = useRouter()
    const toast = useToast()
    return ((res: AxiosResponse, path: string) => {
        if (res.status === 200) {
            // If request successful, check msgCode
            if (res.data.msgCode === 401) {
                toast({title: res.data.t, status: "warning", isClosable: true, position: "top"***REMOVED***)
    ***REMOVED*** else if (res.data.msgCode == 403) {
                toast({title: "Unauthorized", status: "warning", isClosable: true, position: "top"***REMOVED***)
                remove()
                router.push("/login")
                console.debug("Unauthenticated access")
    ***REMOVED***
            return res
***REMOVED*** else if (res.status === 404) {
            toast({title: "404 Not Found", status: "warning", isClosable: true, position: "top"***REMOVED***)
            console.debug(path)
            console.debug("Endpoint " + path + " was not found")
            return res
***REMOVED*** else if (res.status >= 500) {
            toast({title: "Sever Error", status: "error", isClosable: true, position: "top"***REMOVED***)
            console.debug("Server error")
            return res
***REMOVED*** else {
            return res
***REMOVED***
***REMOVED***)
***REMOVED***

export function useApi() {
    const handler = useResponseHandler()
    return {
        get: async (path: string, headers?: THeader): Promise<AxiosResponse> =>
            await axios.get(path, {
                headers: headers
    ***REMOVED***).then(res => {
                return handler(res, path)
    ***REMOVED***),
        post: async (path: string, data: TData, headers?: THeader): Promise<AxiosResponse> =>
            await axios.post(path, data, {
                headers: headers
    ***REMOVED***).then(res => {
                return handler(res, path)
    ***REMOVED***),
        put: async (path: string, data: TData, headers?: THeader): Promise<AxiosResponse> =>
            await axios.put(path, data, {
                headers: headers
    ***REMOVED***).then(res => {
                return handler(res, path)
    ***REMOVED***),
        del: async (path: string, headers?: THeader): Promise<AxiosResponse> =>
            await axios.delete(path, {
                headers: headers
    ***REMOVED***).then(res => {
                return handler(res, path)
    ***REMOVED***),
***REMOVED***
***REMOVED***

export default useApi
