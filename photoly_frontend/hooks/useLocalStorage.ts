import { useCallback } from "react"

export const TOKEN_KEY = "HRD-Token"

const useLocalStorage = (key: string) => {
    const getLS = useCallback(() => localStorage.getItem(key), [key])
    const setLS = useCallback(
        (val: string) => localStorage.setItem(key, val),
        [key]
    )
    const removeLS = useCallback(() => localStorage.removeItem(key), [key])
    return { getLS, setLS, removeLS }
}

export default useLocalStorage