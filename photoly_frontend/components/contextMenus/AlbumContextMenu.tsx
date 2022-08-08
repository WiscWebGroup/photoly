import { DeleteIcon, EditIcon ***REMOVED*** from "@chakra-ui/icons"
import { MenuList, MenuItem ***REMOVED*** from "@chakra-ui/react"
import { GrUpdate ***REMOVED*** from "react-icons/gr"

interface AlbumContextMenuProps {
    handleDelete: () => void
    handleRename: () => void
    handleChangeIcon: () => void
***REMOVED***

const AlbumContextMenu = ({ handleDelete, handleRename, handleChangeIcon ***REMOVED*** : AlbumContextMenuProps) => {
    return (
        <MenuList zIndex={1020***REMOVED***>
            <MenuItem icon={<DeleteIcon />***REMOVED*** onClick={handleDelete***REMOVED***>
                Delete
            </MenuItem>
            <MenuItem icon={<EditIcon />***REMOVED*** onClick={handleRename***REMOVED***>
                Rename
            </MenuItem>
            <MenuItem icon={<GrUpdate />***REMOVED*** onClick={handleChangeIcon***REMOVED***>
                Change Icon
            </MenuItem>
        </MenuList>
    )
***REMOVED***

export default AlbumContextMenu