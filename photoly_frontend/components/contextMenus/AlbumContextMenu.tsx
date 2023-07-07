import { DeleteIcon, EditIcon } from "@chakra-ui/icons"
import { MenuList, MenuItem } from "@chakra-ui/react"
import { GrUpdate } from "react-icons/gr"

interface AlbumContextMenuProps {
    handleDelete: () => void
    handleRename: () => void
    handleChangeIcon: () => void
}

const AlbumContextMenu = ({ handleDelete, handleRename, handleChangeIcon } : AlbumContextMenuProps) => {
    return (
        <MenuList zIndex={1020}>
            <MenuItem icon={<DeleteIcon />} onClick={handleDelete}>
                Delete
            </MenuItem>
            <MenuItem icon={<EditIcon />} onClick={handleRename}>
                Rename
            </MenuItem>
            <MenuItem icon={<GrUpdate />} onClick={handleChangeIcon}>
                Change Icon
            </MenuItem>
        </MenuList>
    )
}

export default AlbumContextMenu