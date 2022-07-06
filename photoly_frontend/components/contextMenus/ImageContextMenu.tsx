import {Menu, MenuItem, MenuList} from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon, DeleteIcon} from "@chakra-ui/icons";
import {Box} from "@chakra-ui/react";
import React from "react";

interface ImageContextProps {
    contextMenuRef: React.LegacyRef<HTMLDivElement>
    x: number,
    y: number
}
const ImageContextMenu = () => {
    return (
        <MenuList>
            <MenuItem icon={<DeleteIcon />}>
                Delete
            </MenuItem>
            <MenuItem icon={<ViewIcon />}>
                View
            </MenuItem>
            <MenuItem icon={<DownloadIcon />}>
                Download
            </MenuItem>
            <MenuItem icon={<EditIcon />}>
                Rename
            </MenuItem>
            <MenuItem icon={<ArrowForwardIcon />}>
                Move to...
            </MenuItem>
        </MenuList>
    )
}
export default ImageContextMenu