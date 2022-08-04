import { MenuList, MenuItem ***REMOVED*** from "@chakra-ui/react"
import { AiOutlineCloudUpload, AiOutlineFolderAdd ***REMOVED*** from "react-icons/ai"

const PanelContextMenu = () => {
    return (
        <MenuList>
            <MenuItem icon={<AiOutlineCloudUpload />***REMOVED***>
                Upload Photo
            </MenuItem>
            <MenuItem icon={<AiOutlineFolderAdd />***REMOVED***>
                New Folder
            </MenuItem>
        </MenuList>
    )
***REMOVED***

export default PanelContextMenu