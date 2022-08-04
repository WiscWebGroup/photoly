import {Box, Button, Flex, Text} from "@chakra-ui/react";
import React from "react";
import {AiOutlineFolderOpen} from 'react-icons/ai'
import { ContextMenu } from "./ContextMenu";
import PanelContextMenu from "./contextMenus/PanelContextMenu";
import PanelContextProvider from "./contexts/PanelContext";
import ImageItem from "./ImageItem";

const Images: React.FC = () => {
    return (
        <ContextMenu<HTMLDivElement>
            renderMenu={() => <PanelContextMenu />}
            // stopPropagation
        >
            {ref => (
                <Box ref={ref} w='100%' h='100%'>
                    <Text alignSelf={"flex-start"} fontSize={"xl"} fontWeight={"semibold"} pl={2}>Folders</Text>
                    <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                        <Box w={64} m={2}>
                            <Button leftIcon={<AiOutlineFolderOpen />} colorScheme='gray' variant='solid' w={"100%"}>
                                Folder 1
                            </Button>
                        </Box>
                        <Box w={64} m={2}>
                            <Button leftIcon={<AiOutlineFolderOpen />} colorScheme='gray' variant='solid' w={"100%"}>
                                Folder 2
                            </Button>
                        </Box>
                        <Box w={64} m={2}>
                            <Button leftIcon={<AiOutlineFolderOpen />} colorScheme='gray' variant='solid' w={"100%"}>
                                Folder 3
                            </Button>
                        </Box>
                    </Flex>
                    <Text alignSelf={"flex-start"} fontSize={"xl"} fontWeight={"semibold"} pl={2}>Images</Text>
                    <PanelContextProvider>

                    </PanelContextProvider>
                    <Flex w={"100%"} justifyContent={"flex-start"} wrap={"wrap"}>
                        <ImageItem src="https://bit.ly/2Z4KKcF" alt="image" />
                        <ImageItem src="https://bit.ly/2Z4KKcF" alt="image" />
                    </Flex>
                </Box>
            )}
        </ContextMenu>
    )
}
export default Images