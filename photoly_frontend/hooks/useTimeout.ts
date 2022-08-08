import { useCallback, useEffect, useRef ***REMOVED*** from "react"

export default function useTimeout(callback: () => void, delay: number) {
  const callbackRef = useRef(callback)
  const timeoutRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    callbackRef.current = callback
  ***REMOVED***, [callback])

  const set = useCallback(() => {
    timeoutRef.current = setTimeout(() => callbackRef.current(), delay)
  ***REMOVED***, [delay])

  const clear = useCallback(() => {
    timeoutRef.current && clearTimeout(timeoutRef.current)
  ***REMOVED***, [])

  useEffect(() => {
    set()
    return clear
  ***REMOVED***, [delay, set, clear])

  const reset = useCallback(() => {
    clear()
    set()
  ***REMOVED***, [clear, set])

  return { reset, clear ***REMOVED***
***REMOVED***