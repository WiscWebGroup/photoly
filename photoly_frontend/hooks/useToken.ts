import useLocalStorage, {TOKEN_KEY***REMOVED*** from "./useLocalStorage";
import {useEffect, useState***REMOVED*** from "react";
import {useRouter***REMOVED*** from "next/router";

const useToken = () => {
    const router = useRouter()
    const {getLS***REMOVED*** = useLocalStorage(TOKEN_KEY)
    const [token, setToken] = useState<string>("")
    useEffect(() => {
        const t = getLS()
        if (t === null){
            router.push("/login")
***REMOVED***else{
            setToken(t)
***REMOVED***
***REMOVED***, [])
    return token
***REMOVED***
export default useToken