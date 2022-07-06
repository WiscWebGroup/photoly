import { DeleteIcon, EditIcon } from "@chakra-ui/icons"
import { MenuList, MenuItem } from "@chakra-ui/react"

const TagContextMenu = () => {
    return (
        <MenuList>
            <MenuItem icon={<DeleteIcon />}>
                Rename
            </MenuItem>
            <MenuItem icon={<EditIcon />}>
                Delete
            </MenuItem>
        </MenuList>
    )
}

export default TagContextMenu