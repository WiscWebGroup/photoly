import { MenuList, MenuItem ***REMOVED*** from "@chakra-ui/react"
import { AiOutlineCloudUpload, AiOutlineFolderAdd ***REMOVED*** from "react-icons/ai"

interface PanelContextMenuProps {
    triggerUpload: () => void
    triggerCreate: () => void
***REMOVED***

const PanelContextMenu = ({ triggerUpload, triggerCreate ***REMOVED***: PanelContextMenuProps) => {
    return (
        <MenuList zIndex={1020***REMOVED***>
            <MenuItem icon={<AiOutlineCloudUpload />***REMOVED*** onClick={triggerUpload***REMOVED***>
                Upload Photo
            </MenuItem>
            <MenuItem icon={<AiOutlineFolderAdd />***REMOVED*** onClick={triggerCreate***REMOVED***>
                New Folder
            </MenuItem>
        </MenuList>
    )
***REMOVED***

export default PanelContextMenu