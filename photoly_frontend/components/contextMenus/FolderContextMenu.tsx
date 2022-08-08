import { DeleteIcon, EditIcon, ArrowForwardIcon ***REMOVED*** from "@chakra-ui/icons"
import { MenuList, MenuItem ***REMOVED*** from "@chakra-ui/react"

interface FolderContextMenuProps {
    handleDelete: () => void
    handleRename: () => void
    handleMoveTo: () => void
***REMOVED***

const FolderContextMenu = ({ handleDelete, handleRename, handleMoveTo ***REMOVED***: FolderContextMenuProps) => {
    return (
        <MenuList zIndex={1030***REMOVED***>
            <MenuItem icon={<DeleteIcon />***REMOVED*** onClick={handleDelete***REMOVED***>
                Delete
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

export default FolderContextMenu