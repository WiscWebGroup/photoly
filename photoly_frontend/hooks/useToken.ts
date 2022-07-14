import useLocalStorage, {TOKEN_KEY} from "./useLocalStorage";
import {useEffect, useState} from "react";
import {useRouter} from "next/router";

const useToken = () => {
    const router = useRouter()
    const {getLS} = useLocalStorage(TOKEN_KEY)
    const [token, setToken] = useState<string>("")
    useEffect(() => {
        const t = getLS()
        if (t === null){
            router.push("/login")
        }else{
            setToken(t)
        }
    }, [])
    return token
}
export default useToken