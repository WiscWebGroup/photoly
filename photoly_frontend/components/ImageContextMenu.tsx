import {Menu, MenuItem, MenuList***REMOVED*** from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon***REMOVED*** from "@chakra-ui/icons";
import {Box***REMOVED*** from "@chakra-ui/react";
import React from "react";

interface ImageContextProps {
    x: number,
    y: number
***REMOVED***
const ImageContextMenu: React.FC<ImageContextProps> = ({x, y***REMOVED***) => {
    return (
        <Box position={"absolute"***REMOVED*** top={y***REMOVED*** left={x***REMOVED***>
            <Menu isOpen={true***REMOVED***>
                <MenuList>
                    <MenuItem icon={<ViewIcon />***REMOVED***>
                        View
                    </MenuItem>
                    <MenuItem icon={<DownloadIcon />***REMOVED***>
                        Download
                    </MenuItem>
                    <MenuItem icon={<EditIcon />***REMOVED***>
                        Rename
                    </MenuItem>
                    <MenuItem icon={<ArrowForwardIcon />***REMOVED***>
                        Move to...
                    </MenuItem>
                </MenuList>
            </Menu>
        </Box>

    )
***REMOVED***
export default ImageContextMenu