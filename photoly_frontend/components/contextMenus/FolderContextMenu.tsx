import { DeleteIcon, EditIcon, ArrowForwardIcon } from "@chakra-ui/icons"
import { MenuList, MenuItem } from "@chakra-ui/react"

interface FolderContextMenuProps {
    handleDelete: () => void
    handleRename: () => void
    handleMoveTo: () => void
}

const FolderContextMenu = ({ handleDelete, handleRename, handleMoveTo }: FolderContextMenuProps) => {
    return (
        <MenuList zIndex={1030}>
            <MenuItem icon={<DeleteIcon />} onClick={handleDelete}>
                Delete
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

export default FolderContextMenu