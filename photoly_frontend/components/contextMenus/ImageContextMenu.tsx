import {MenuItem, MenuList} from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon, DeleteIcon} from "@chakra-ui/icons";
import React from "react";

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