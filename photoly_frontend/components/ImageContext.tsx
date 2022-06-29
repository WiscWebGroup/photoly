import {Menu, MenuButton, MenuItem, MenuList} from "@chakra-ui/menu";
import {AddIcon, EditIcon, ExternalLinkIcon, HamburgerIcon, RepeatIcon} from "@chakra-ui/icons";
import {Box, IconButton} from "@chakra-ui/react";

interface props {
    x: number,
    y: number
}
export default function ImageContext({x, y}: props) {
    return (
        <Box position={"absolute"} top={y} left={x}>
            <Menu isOpen={true}>
                <MenuList>
                    <MenuItem icon={<AddIcon />} command='⌘T'>
                        New Tab
                    </MenuItem>
                    <MenuItem icon={<ExternalLinkIcon />} command='⌘N'>
                        New Window
                    </MenuItem>
                    <MenuItem icon={<RepeatIcon />} command='⌘⇧N'>
                        Open Closed Tab
                    </MenuItem>
                    <MenuItem icon={<EditIcon />} command='⌘O'>
                        Open File...
                    </MenuItem>
                </MenuList>
            </Menu>
        </Box>

    )
}