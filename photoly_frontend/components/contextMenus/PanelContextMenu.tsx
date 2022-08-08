import { MenuList, MenuItem } from "@chakra-ui/react"
import { AiOutlineCloudUpload, AiOutlineFolderAdd } from "react-icons/ai"

interface PanelContextMenuProps {
    triggerUpload: () => void
    triggerCreate: () => void
}

const PanelContextMenu = ({ triggerUpload, triggerCreate }: PanelContextMenuProps) => {
    return (
        <MenuList zIndex={1020}>
            <MenuItem icon={<AiOutlineCloudUpload />} onClick={triggerUpload}>
                Upload Photo
            </MenuItem>
            <MenuItem icon={<AiOutlineFolderAdd />} onClick={triggerCreate}>
                New Folder
            </MenuItem>
        </MenuList>
    )
}

export default PanelContextMenu