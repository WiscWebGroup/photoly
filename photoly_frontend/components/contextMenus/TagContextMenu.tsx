import { DeleteIcon, EditIcon } from "@chakra-ui/icons"
import { MenuList, MenuItem } from "@chakra-ui/react"

interface TagContextMenuProps {
    handleRename: () => void
    handleDelete: () => void
}

const TagContextMenu = ({ handleRename, handleDelete }: TagContextMenuProps) => {
    return (
        <MenuList>
            <MenuItem icon={<DeleteIcon />} onClick={handleDelete}>
                Delete
            </MenuItem>
            <MenuItem icon={<EditIcon />} onClick={handleRename}>
                Rename
            </MenuItem>
        </MenuList>
    )
}

export default TagContextMenu