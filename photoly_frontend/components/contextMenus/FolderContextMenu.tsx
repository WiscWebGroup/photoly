import { DeleteIcon, EditIcon, ArrowForwardIcon } from "@chakra-ui/icons"
import { MenuList, MenuItem } from "@chakra-ui/react"

const FolderContextMenu = () => {
    return (
        <MenuList>
            <MenuItem icon={<DeleteIcon />}>
                Delete
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

export default FolderContextMenu