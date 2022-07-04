import { useCallback } from "react"

export const TOKEN_KEY = "HRD-Token"

const useLocalStorage = (key: string) => {
    const get = useCallback(() => localStorage.getItem(key), [key])
    const set = useCallback(
        (val: string) => localStorage.setItem(key, val),
        [key]
    )
    const remove = useCallback(() => localStorage.removeItem(key), [key])
    return { get, set, remove }
}

export default useLocalStorage