import {Menu, MenuItem, MenuList} from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon} from "@chakra-ui/icons";
import {Box} from "@chakra-ui/react";

interface props {
    x: number,
    y: number
}
export default function ImageContext({x, y}: props) {
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