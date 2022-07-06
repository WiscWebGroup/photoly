import {Menu, MenuItem, MenuList***REMOVED*** from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon, DeleteIcon***REMOVED*** from "@chakra-ui/icons";
import {Box***REMOVED*** from "@chakra-ui/react";
import React from "react";

interface ImageContextProps {
    contextMenuRef: React.LegacyRef<HTMLDivElement>
    x: number,
    y: number
***REMOVED***
const ImageContextMenu = () => {
    return (
        <MenuList>
            <MenuItem icon={<DeleteIcon />***REMOVED***>
                Delete
            </MenuItem>
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
    )
***REMOVED***
export default ImageContextMenu