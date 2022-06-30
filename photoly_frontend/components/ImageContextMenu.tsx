import {Menu, MenuItem, MenuList***REMOVED*** from "@chakra-ui/menu";
import {ArrowForwardIcon, DownloadIcon, EditIcon, ViewIcon***REMOVED*** from "@chakra-ui/icons";
import {Box***REMOVED*** from "@chakra-ui/react";

interface ImageContextProps {
    x: number,
    y: number
***REMOVED***
export default function ImageContextMenu({x, y***REMOVED***: ImageContextProps) {
    return (
        <Box position={"absolute"***REMOVED*** top={y***REMOVED*** left={x***REMOVED***>
            <Menu isOpen={true***REMOVED***>
                <MenuList>
                    <MenuItem icon={<ViewIcon />***REMOVED***>
                        View
                    </MenuItem>
                    <MenuItem icon={<DownloadIcon />***REMOVED***>
                        Download
                    </MenuItem>
                    <MenuItem icon={<EditIcon />***REMOVED***>
                        Rename
                    </MenuItem>
                    <MenuItem icon={<ArrowForwardIcon />***REMOVED***>
                        Move to...
                    </MenuItem>
                </MenuList>
            </Menu>
        </Box>

    )
***REMOVED***