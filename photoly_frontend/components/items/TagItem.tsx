import { Box, Checkbox, CheckboxGroup, Input, useDisclosure, useOutsideClick } from "@chakra-ui/react";
import React, { ChangeEvent, KeyboardEvent, useRef, useState } from "react";
import { ContextMenu } from "../ContextMenu";
import TagContextMenu from "../contextMenus/TagContextMenu";
import { useSearchFilter, useSearchUpdateDispatch } from "../contexts/SearchContext";
import { useTagListUpdate } from "../contexts/TagContext";

interface TagItemProps {
    tagId: number
    tagName: string
}

 const TagItem: React.FC<TagItemProps> = ({tagId, tagName}) => {
    const [tag, setTag] = useState(tagName)
    const inputRef = useRef<HTMLInputElement>(null)
    const checkboxRef = useRef<HTMLInputElement>(null)
    const { isOpen, onOpen, onClose } = useDisclosure()  // control edit mode
    const { renameRequest, deleteRequest } = useTagListUpdate()
    const { tagIds: searchTagIds } = useSearchFilter()
    const [isChecked, setIsChecked] = useState(false)
    const searchUpdateDispatch = useSearchUpdateDispatch()

    const handleRename = () => {
        onOpen()
        inputRef.current?.focus()
    }

    const callThenClose = () => {
        // empty or unmodified
        if (tag.length === 0 || tag === tagName) {
            setTag(tagName)
            onClose()
            return
        }

        // modified
        renameRequest({id: tagId, name: tag})
        onClose()
    }

    const handleEnterClose = (e: KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter')
            callThenClose()
    }

    useOutsideClick({
        ref: inputRef,
        handler: callThenClose
    })

    const handleDelete = () => {
        deleteRequest(tagId)
    }

    const handleChecked = (e: ChangeEvent<HTMLInputElement>) => {
        if (e.currentTarget.checked)
            searchUpdateDispatch({type: "add_tag", payload: tagId})
        else
            searchUpdateDispatch({type: "remove_tag", payload: tagId})
    }

    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() =>
                <TagContextMenu 
                    handleRename={handleRename}
                    handleDelete={handleDelete}
                />}
        >
            {ref => (
                <Box ref={ref} w='100%' pl={4} display='flex'>
                    <CheckboxGroup value={searchTagIds}>
                        <Checkbox 
                            ref={checkboxRef}
                            variant='ghost' 
                            value={tagId}
                            pr={2} 
                            onChange={handleChecked}
                        >
                            {isOpen ? '' : tag}
                        </Checkbox>
                        {isOpen && 
                            <Input 
                                ref={inputRef}
                                variant='flushed'
                                value={tag}
                                autoFocus
                                onKeyDown={handleEnterClose}
                                onChange={(e) => setTag(e.currentTarget.value)}
                            />
                        }
                    </CheckboxGroup>
                </Box>
            )}
        </ContextMenu>
        
    )
}
export default TagItem