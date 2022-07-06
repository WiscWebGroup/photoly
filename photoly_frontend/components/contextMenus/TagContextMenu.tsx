import { DeleteIcon, EditIcon ***REMOVED*** from "@chakra-ui/icons"
import { MenuList, MenuItem ***REMOVED*** from "@chakra-ui/react"

const TagContextMenu = () => {
    return (
        <MenuList>
            <MenuItem icon={<DeleteIcon />***REMOVED***>
                Rename
            </MenuItem>
            <MenuItem icon={<EditIcon />***REMOVED***>
                Delete
            </MenuItem>
        </MenuList>
    )
***REMOVED***

export default TagContextMenu