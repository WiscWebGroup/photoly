import {MouseEvent, useCallback, useEffect, useState***REMOVED*** from "react";

interface Coordinate {
    x: number,
    y: number
***REMOVED***
export const useContextMenu = (): [boolean, (e: MouseEvent) => void, Coordinate] => {
    const [showContext, setShowContext] = useState<boolean>(false)
    const [anchorPoint, setAnchorPoint] = useState<Coordinate>({ x: 0, y: 0 ***REMOVED***)
    const handleClick = useCallback(() => (showContext ? setShowContext(false) : null), [showContext])
    const handleContextMenu = (e: MouseEvent) => {
        e.preventDefault()
        setAnchorPoint({ x: e.pageX, y: e.pageY ***REMOVED***)
        setShowContext(true)
***REMOVED***
    // Hide menu when clicking outside this image
    useEffect(() => {
        document.addEventListener("click", handleClick);
        return () => {
            document.removeEventListener("click", handleClick);
***REMOVED***
***REMOVED***)

    return [showContext, handleContextMenu, anchorPoint]
***REMOVED***