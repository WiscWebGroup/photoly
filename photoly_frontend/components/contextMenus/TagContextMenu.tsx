import { DeleteIcon, EditIcon ***REMOVED*** from "@chakra-ui/icons"
import { MenuList, MenuItem ***REMOVED*** from "@chakra-ui/react"

interface TagContextMenuProps {
    handleRename: () => void
    handleDelete: () => void
***REMOVED***

const TagContextMenu = ({ handleRename, handleDelete ***REMOVED***: TagContextMenuProps) => {
    return (
        <MenuList>
            <MenuItem icon={<DeleteIcon />***REMOVED*** onClick={handleDelete***REMOVED***>
                Delete
            </MenuItem>
            <MenuItem icon={<EditIcon />***REMOVED*** onClick={handleRename***REMOVED***>
                Rename
            </MenuItem>
        </MenuList>
    )
***REMOVED***

export default TagContextMenu