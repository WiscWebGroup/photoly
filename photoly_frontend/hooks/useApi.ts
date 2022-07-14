import axios, {AxiosError, AxiosRequestConfig, AxiosRequestHeaders, AxiosResponse***REMOVED*** from "axios";
import useLocalStorage, {TOKEN_KEY***REMOVED*** from "./useLocalStorage";
import {useRouter***REMOVED*** from "next/router";
import {useBoolean, useToast***REMOVED*** from "@chakra-ui/react";

export type TData = { [key: string]: any ***REMOVED*** | null
export type THeader = AxiosRequestHeaders | undefined

const defaultHeaders = {
    "Content-Type": "application/json",
    Accept: "application/json"
***REMOVED***

// Handles status code == 200
function useResponseHandler() {
    const {removeLS***REMOVED*** = useLocalStorage(TOKEN_KEY)
    const router = useRouter()
    const toast = useToast()
    return ((res: AxiosResponse) => {
        // If request successful, check msgCode
        if (res.data.msgCode === 401 || res.data.msgCode === 400) {
            if (res.data.t === null){
                toast({title: "Something is wrong...", status: "warning", isClosable: true, position: "top"***REMOVED***)
    ***REMOVED*** else {
                toast({title: res.data.t, status: "warning", isClosable: true, position: "top"***REMOVED***)
    ***REMOVED***
***REMOVED*** else if (res.data.msgCode == 403) {
            toast({title: "Unauthorized", status: "warning", isClosable: true, position: "top"***REMOVED***)
            removeLS()
            router.push("/login")
            console.debug("Unauthenticated access")
***REMOVED***
        return res
***REMOVED***)
***REMOVED***

// Handles status code != 200
function useErrorHandler() {
    const toast = useToast()
    return ((err: AxiosError, path: string) => {
        if (!!err.response && err.response.status === 404) {
            toast({title: "404 Not Found", status: "warning", isClosable: true, position: "top"***REMOVED***)
            console.debug(path)
            console.debug("Endpoint " + path + " was not found")
            return err
***REMOVED*** else if (!!err.response && err.response.status === 500) {
            console.debug(err)
            toast({title: "Sever Error", status: "error", isClosable: true, position: "top"***REMOVED***)
            console.debug("Server error")
            return err
***REMOVED***
***REMOVED***)
***REMOVED***

export function useApi() {
    const respHandler = useResponseHandler()
    const errHandler = useErrorHandler()
    const [isLoading, setLoading] = useBoolean(false)
    return {
        isLoading,
        get: async (path: string, config: AxiosRequestConfig): Promise<AxiosResponse | void> => {
            setLoading.on()
            return await axios.get(path, config).then(res => {
                setLoading.off()
                return respHandler(res)
    ***REMOVED***).catch(err => {
                setLoading.off()
                errHandler(err, path)
    ***REMOVED***)
***REMOVED***,
        post: async (path: string, data: TData, config: AxiosRequestConfig): Promise<AxiosResponse | void> => {
            setLoading.on()
            return await axios.post(path, data, config).then(res => {
                setLoading.off()
                return respHandler(res)
    ***REMOVED***).catch(err => {
                setLoading.off()
                errHandler(err, path)
    ***REMOVED***)
***REMOVED***,
        put: async (path: string, data: TData, config: AxiosRequestConfig): Promise<AxiosResponse | void> => {
            setLoading.on()
            return await axios.put(path, data, config).then(res => {
                setLoading.off()
                return respHandler(res)
    ***REMOVED***).catch(err => {
                setLoading.off()
                errHandler(err, path)
    ***REMOVED***)
***REMOVED***,
        del: async (path: string, config: AxiosRequestConfig): Promise<AxiosResponse | void> => {
            setLoading.on()
            return await axios.delete(path, config).then(res => {
                setLoading.off()
                return respHandler(res)
    ***REMOVED***).catch(err => {
                setLoading.off()
                errHandler(err, path)
    ***REMOVED***)
***REMOVED***,
***REMOVED***
***REMOVED***

export default useApi
