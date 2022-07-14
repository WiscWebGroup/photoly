import { Box, Checkbox ***REMOVED*** from "@chakra-ui/react";
import { ContextMenu ***REMOVED*** from "chakra-ui-contextmenu";
import React from "react";
import TagContextMenu from "./contextMenus/TagContextMenu";

interface TagItemProps {
    tagName: string
***REMOVED***

 const TagItem: React.FC<TagItemProps> = ({tagName***REMOVED***) => {
    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() => (<TagContextMenu />)***REMOVED***
        >
        ***REMOVED***ref => (
                <Box ref={ref***REMOVED*** w='100%'>
                    <Checkbox variant='ghost' w='100%'>{tagName***REMOVED***</Checkbox >
                </Box>
            )***REMOVED***
        </ContextMenu>
        
    )
***REMOVED***
export default TagItem