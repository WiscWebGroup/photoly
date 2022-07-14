import { Box, Checkbox, Input, useDisclosure, useOutsideClick } from "@chakra-ui/react";
import { ContextMenu } from "chakra-ui-contextmenu";
import React, { KeyboardEvent, useRef, useState } from "react";
import TagContextMenu from "./contextMenus/TagContextMenu";
import { useTagListUpdate } from "./contexts/TagContext";

interface TagItemProps {
    tagId: number
    tagName: string
}

 const TagItem: React.FC<TagItemProps> = ({tagId, tagName}) => {
    const [tag, setTag] = useState(tagName)
    const inputRef = useRef<HTMLInputElement>(null)
    const { isOpen, onOpen, onClose } = useDisclosure()  // control edit mode
    const { renameRequest, deleteRequest } = useTagListUpdate()

    const handleRename = () => {
        onOpen()
        inputRef.current?.focus()
    }

    const callThenClose = () => {
        if (tag.length === 0)
            setTag(tagName)
        renameRequest({tagId, tagName: tag})
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

    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() => (
                <TagContextMenu 
                    handleRename={handleRename}
                    handleDelete={handleDelete}
                />
            )
        }
        >
            {ref => (
                <Box ref={ref} w='100%' pl={4} display='flex'>
                    <Checkbox variant='ghost' pr={2}>{isOpen ? '' : tag}</Checkbox>
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
                </Box>
            )}
        </ContextMenu>
        
    )
}
export default TagItem