import { DeleteIcon, EditIcon, ArrowForwardIcon ***REMOVED*** from "@chakra-ui/icons"
import { MenuList, MenuItem ***REMOVED*** from "@chakra-ui/react"

const FolderContextMenu = () => {
    return (
        <MenuList>
            <MenuItem icon={<DeleteIcon />***REMOVED***>
                Delete
            </MenuItem>
            <MenuItem icon={<EditIcon />***REMOVED***>
                Rename
            </MenuItem>
            <MenuItem icon={<ArrowForwardIcon />***REMOVED***>
                Move to...
            </MenuItem>
        </MenuList>
    )
***REMOVED***

export default FolderContextMenu