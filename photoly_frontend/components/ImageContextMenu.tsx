import {Menu, MenuItem, MenuList} from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon} from "@chakra-ui/icons";
import {Box} from "@chakra-ui/react";
import React from "react";

interface ImageContextProps {
    x: number,
    y: number
}
const ImageContextMenu: React.FC<ImageContextProps> = ({x, y}) => {
    return (
        <Box position={"absolute"} top={y} left={x}>
            <Menu isOpen={true}>
                <MenuList>
                    <MenuItem icon={<ViewIcon />}>
                        View
                    </MenuItem>
                    <MenuItem icon={<DownloadIcon />}>
                        Download
                    </MenuItem>
                    <MenuItem icon={<EditIcon />}>
                        Rename
                    </MenuItem>
                    <MenuItem icon={<ArrowForwardIcon />}>
                        Move to...
                    </MenuItem>
                </MenuList>
            </Menu>
        </Box>

    )
}
export default ImageContextMenu