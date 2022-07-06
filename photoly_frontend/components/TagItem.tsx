import { Box, Checkbox } from "@chakra-ui/react";
import { ContextMenu } from "chakra-ui-contextmenu";
import React from "react";
import TagContextMenu from "./contextMenus/TagContextMenu";

interface TagItemProps {
    tagName: string
}

 const TagItem: React.FC<TagItemProps> = ({tagName}) => {
    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() => (<TagContextMenu />)}
        >
            {ref => (
                <Box ref={ref} w='100%'>
                    <Checkbox variant='ghost' w='100%'>{tagName}</Checkbox >
                </Box>
            )}
        </ContextMenu>
        
    )
}
export default TagItem