import { MenuList, MenuItem } from "@chakra-ui/react"
import { AiOutlineCloudUpload, AiOutlineFolderAdd } from "react-icons/ai"

const PanelContextMenu = () => {
    return (
        <MenuList zIndex={1020}>
            <MenuItem icon={<AiOutlineCloudUpload />}>
                Upload Photo
            </MenuItem>
            <MenuItem icon={<AiOutlineFolderAdd />}>
                New Folder
            </MenuItem>
        </MenuList>
    )
}

export default PanelContextMenu