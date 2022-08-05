import {Box, Button, Flex, Text***REMOVED*** from "@chakra-ui/react";
import React from "react";
import {AiOutlineFolderOpen***REMOVED*** from 'react-icons/ai'
import { ContextMenu ***REMOVED*** from "./ContextMenu";
import PanelContextMenu from "./contextMenus/PanelContextMenu";

import ImageItem from "./ImageItem";

const Images: React.FC = () => {
    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() => <PanelContextMenu />***REMOVED***
            // stopPropagation
        >
   ***REMOVED*****REMOVED***ref => (
                <Box ref={ref***REMOVED*** w='100%' h='100%'>
                    <Text alignSelf={"flex-start"***REMOVED*** fontSize={"xl"***REMOVED*** fontWeight={"semibold"***REMOVED*** pl={2***REMOVED***>Folders</Text>
                    <Flex w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED*** wrap={"wrap"***REMOVED***>
                        <Box w={64***REMOVED*** m={2***REMOVED***>
                            <Button leftIcon={<AiOutlineFolderOpen />***REMOVED*** colorScheme='gray' variant='solid' w={"100%"***REMOVED***>
                                Folder 1
                            </Button>
                        </Box>
                        <Box w={64***REMOVED*** m={2***REMOVED***>
                            <Button leftIcon={<AiOutlineFolderOpen />***REMOVED*** colorScheme='gray' variant='solid' w={"100%"***REMOVED***>
                                Folder 2
                            </Button>
                        </Box>
                        <Box w={64***REMOVED*** m={2***REMOVED***>
                            <Button leftIcon={<AiOutlineFolderOpen />***REMOVED*** colorScheme='gray' variant='solid' w={"100%"***REMOVED***>
                                Folder 3
                            </Button>
                        </Box>
                    </Flex>
                    <Text alignSelf={"flex-start"***REMOVED*** fontSize={"xl"***REMOVED*** fontWeight={"semibold"***REMOVED*** pl={2***REMOVED***>Images</Text>
                    <Flex w={"100%"***REMOVED*** justifyContent={"flex-start"***REMOVED*** wrap={"wrap"***REMOVED***>
                        <ImageItem src="https://bit.ly/2Z4KKcF" alt="image" />
                        <ImageItem src="https://bit.ly/2Z4KKcF" alt="image" />
                    </Flex>
                </Box>
            )***REMOVED***
        </ContextMenu>
    )
***REMOVED***
export default Images