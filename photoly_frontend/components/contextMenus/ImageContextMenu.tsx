import {MenuItem, MenuList} from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon, DeleteIcon} from "@chakra-ui/icons";
import React from "react";

interface ImageContextMenuProps {
    handleDelete: () => void
    handleView: () => void
    handleDownload: () => void
    handleRename: () => void
    handleMoveTo: () => void
}

const ImageContextMenu = ({ handleDelete, handleView, handleDownload, handleRename, handleMoveTo } : ImageContextMenuProps) => {
    return (
        <MenuList zIndex={1030}>
            <MenuItem icon={<DeleteIcon />} onClick={handleDelete}>
                Delete
            </MenuItem>
            <MenuItem icon={<ViewIcon />} onClick={handleView}>
                View
            </MenuItem>
            <MenuItem icon={<DownloadIcon />} onClick={handleDownload}>
                Download
            </MenuItem>
            <MenuItem icon={<EditIcon />} onClick={handleRename}>
                Rename
            </MenuItem>
            <MenuItem icon={<ArrowForwardIcon />} onClick={handleMoveTo}>
                Move to...
            </MenuItem>
        </MenuList>
    )
}

export default ImageContextMenu