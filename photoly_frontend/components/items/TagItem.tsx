import { Box, Checkbox, CheckboxGroup, Input, useDisclosure, useOutsideClick ***REMOVED*** from "@chakra-ui/react";
import React, { ChangeEvent, KeyboardEvent, useRef, useState ***REMOVED*** from "react";
import { ContextMenu ***REMOVED*** from "../ContextMenu";
import TagContextMenu from "../contextMenus/TagContextMenu";
import { useSearchFilter, useSearchUpdateDispatch ***REMOVED*** from "../contexts/SearchContext";
import { useDeleteTagMutation, useRenameTagMutation ***REMOVED*** from "../../redux/api/tagSlice";

interface TagItemProps {
    tagId: number
    tagName: string
***REMOVED***

 const TagItem: React.FC<TagItemProps> = ({tagId, tagName***REMOVED***) => {
    const [tag, setTag] = useState(tagName)
    const inputRef = useRef<HTMLInputElement>(null)
    const checkboxRef = useRef<HTMLInputElement>(null)
    const { isOpen, onOpen, onClose ***REMOVED*** = useDisclosure()  // control edit mode
    const { tagIds: searchTagIds ***REMOVED*** = useSearchFilter()
    const searchUpdateDispatch = useSearchUpdateDispatch()
   const [deleteRequest] = useDeleteTagMutation()
   const [renameRequest] = useRenameTagMutation()

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
        renameRequest({tag_id: tagId, new_name: tag***REMOVED***)
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

    const handleChecked = (e: ChangeEvent<HTMLInputElement>) => {
        if (e.currentTarget.checked)
            searchUpdateDispatch({type: "add_tag", payload: tagId***REMOVED***)
        else
            searchUpdateDispatch({type: "remove_tag", payload: tagId***REMOVED***)
***REMOVED***

    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() =>
                <TagContextMenu 
                    handleRename={handleRename***REMOVED***
                    handleDelete={handleDelete***REMOVED***
                />***REMOVED***
        >
   ***REMOVED*****REMOVED***ref => (
                <Box ref={ref***REMOVED*** w='100%' pl={4***REMOVED*** display='flex'>
                    <CheckboxGroup value={searchTagIds***REMOVED***>
                        <Checkbox 
                            ref={checkboxRef***REMOVED***
                            variant='ghost' 
                            value={tagId***REMOVED***
                            pr={2***REMOVED*** 
                            onChange={handleChecked***REMOVED***
                        >
         ***REMOVED*****REMOVED*****REMOVED*****REMOVED***isOpen ? '' : tagName***REMOVED***
                        </Checkbox>
     ***REMOVED*****REMOVED*****REMOVED*****REMOVED***isOpen && 
                            <Input 
                                ref={inputRef***REMOVED***
                                variant='flushed'
                                value={tag***REMOVED***
                                autoFocus
                                onKeyDown={handleEnterClose***REMOVED***
                                onChange={(e) => setTag(e.currentTarget.value)***REMOVED***
                            />
 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***
                    </CheckboxGroup>
                </Box>
            )***REMOVED***
        </ContextMenu>
        
    )
***REMOVED***
export default TagItem