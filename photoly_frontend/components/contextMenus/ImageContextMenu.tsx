import {MenuItem, MenuList***REMOVED*** from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon, DeleteIcon***REMOVED*** from "@chakra-ui/icons";
import React from "react";

interface ImageContextMenuProps {
    handleDelete: () => void
    handleView: () => void
    handleDownload: () => void
    handleRename: () => void
    handleMoveTo: () => void
***REMOVED***

const ImageContextMenu = ({ handleDelete, handleView, handleDownload, handleRename, handleMoveTo ***REMOVED*** : ImageContextMenuProps) => {
    return (
        <MenuList zIndex={1030***REMOVED***>
            <MenuItem icon={<DeleteIcon />***REMOVED*** onClick={handleDelete***REMOVED***>
                Delete
            </MenuItem>
            <MenuItem icon={<ViewIcon />***REMOVED*** onClick={handleView***REMOVED***>
                View
            </MenuItem>
            <MenuItem icon={<DownloadIcon />***REMOVED*** onClick={handleDownload***REMOVED***>
                Download
            </MenuItem>
            <MenuItem icon={<EditIcon />***REMOVED*** onClick={handleRename***REMOVED***>
                Rename
            </MenuItem>
            <MenuItem icon={<ArrowForwardIcon />***REMOVED*** onClick={handleMoveTo***REMOVED***>
                Move to...
            </MenuItem>
        </MenuList>
    )
***REMOVED***

export default ImageContextMenu