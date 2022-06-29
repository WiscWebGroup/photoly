import {Menu, MenuButton, MenuItem, MenuList***REMOVED*** from "@chakra-ui/menu";
import {AddIcon, EditIcon, ExternalLinkIcon, HamburgerIcon, RepeatIcon***REMOVED*** from "@chakra-ui/icons";
import {Box, IconButton***REMOVED*** from "@chakra-ui/react";

interface props {
    x: number,
    y: number
***REMOVED***
export default function ImageContext({x, y***REMOVED***: props) {
    return (
        <Box position={"absolute"***REMOVED*** top={y***REMOVED*** left={x***REMOVED***>
            <Menu isOpen={true***REMOVED***>
                <MenuList>
                    <MenuItem icon={<AddIcon />***REMOVED*** command='⌘T'>
                        New Tab
                    </MenuItem>
                    <MenuItem icon={<ExternalLinkIcon />***REMOVED*** command='⌘N'>
                        New Window
                    </MenuItem>
                    <MenuItem icon={<RepeatIcon />***REMOVED*** command='⌘⇧N'>
                        Open Closed Tab
                    </MenuItem>
                    <MenuItem icon={<EditIcon />***REMOVED*** command='⌘O'>
                        Open File...
                    </MenuItem>
                </MenuList>
            </Menu>
        </Box>

    )
***REMOVED***