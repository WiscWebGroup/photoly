import { Box, Checkbox, Input, useDisclosure, useOutsideClick ***REMOVED*** from "@chakra-ui/react";
import React, { KeyboardEvent, useRef, useState ***REMOVED*** from "react";
import { ContextMenu ***REMOVED*** from "./ContextMenu";
import TagContextMenu from "./contextMenus/TagContextMenu";
import { useTagListUpdate ***REMOVED*** from "./contexts/TagContext";

interface TagItemProps {
    tagId: number
    tagName: string
***REMOVED***

 const TagItem: React.FC<TagItemProps> = ({tagId, tagName***REMOVED***) => {
    const [tag, setTag] = useState(tagName)
    const inputRef = useRef<HTMLInputElement>(null)
    const { isOpen, onOpen, onClose ***REMOVED*** = useDisclosure()  // control edit mode
    const { renameRequest, deleteRequest ***REMOVED*** = useTagListUpdate()

    const handleRename = () => {
        onOpen()
        inputRef.current?.focus()
***REMOVED***

    const callThenClose = () => {
        // empty or unmodified
        if (tag.length === 0 || tag === tagName) {
            setTag(tagName)
            onClose()
            return
***REMOVED***

        // modified
        renameRequest({id: tagId, name: tag***REMOVED***)
        onClose()
***REMOVED***

    const handleEnterClose = (e: KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter')
            callThenClose()
***REMOVED***

    useOutsideClick({
        ref: inputRef,
        handler: callThenClose
***REMOVED***)

    const handleDelete = () => {
        deleteRequest(tagId)
***REMOVED***

    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() => (
                <TagContextMenu 
                    handleRename={handleRename***REMOVED***
                    handleDelete={handleDelete***REMOVED***
                />
            )
***REMOVED***
        >
   ***REMOVED*****REMOVED***ref => (
                <Box ref={ref***REMOVED*** w='100%' pl={4***REMOVED*** display='flex'>
                    <Checkbox variant='ghost' pr={2***REMOVED***>{isOpen ? '' : tag***REMOVED***</Checkbox>
 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***isOpen && 
                    <Input 
                        ref={inputRef***REMOVED***
                        variant='flushed'
                        value={tag***REMOVED***
                        autoFocus
                        onKeyDown={handleEnterClose***REMOVED***
                        onChange={(e) => setTag(e.currentTarget.value)***REMOVED***
                    />
  ***REMOVED*****REMOVED*****REMOVED***
                </Box>
            )***REMOVED***
        </ContextMenu>
        
    )
***REMOVED***
export default TagItem